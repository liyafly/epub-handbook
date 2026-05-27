import { parseXml, elements, firstElement, normalizeText } from "./xml.js";
import { formatBytes } from "../util/format.js";
import { IncrementalSha256 } from "../util/hash.js";

function requireZip() {
  if (!globalThis.zip?.ZipReader) {
    throw new Error("zip.js vendor asset is missing. Run tools/epub-diff/scripts/fetch-vendor.sh first.");
  }
  return globalThis.zip;
}

function joinPath(base, href) {
  const parts = `${base}/${href}`.split("/");
  const out = [];
  for (const part of parts) {
    if (!part || part === ".") continue;
    if (part === "..") out.pop();
    else out.push(part);
  }
  return out.join("/");
}

export async function readEntryText(entry) {
  return entry.getData(new (requireZip()).TextWriter());
}

export async function readEntryBuffer(entry) {
  const blob = await entry.getData(new (requireZip()).BlobWriter());
  return blob.arrayBuffer();
}

export async function readEntryHash(entry) {
  const zipLib = requireZip();
  class HashWriter extends zipLib.Writer {
    constructor() {
      super();
      this.hasher = new IncrementalSha256();
      this.size = 0;
    }

    writeUint8Array(chunk) {
      this.size += chunk.length;
      this.hasher.update(chunk);
    }

    getData() {
      return { sha256: this.hasher.digest(), size: this.size };
    }
  }
  return entry.getData(new HashWriter());
}

function manifestItems(opfDoc, opfDir) {
  return elements(opfDoc, "item").map((item) => {
    const href = item.getAttribute("href") || "";
    return {
      id: item.getAttribute("id") || "",
      href,
      path: joinPath(opfDir, href),
      mediaType: item.getAttribute("media-type") || "",
      properties: item.getAttribute("properties") || "",
    };
  });
}

function spineIdrefs(opfDoc) {
  return elements(opfDoc, "itemref").map((item) => item.getAttribute("idref") || "");
}

function metadata(opfDoc) {
  const fields = [];
  for (const node of elements(opfDoc, "metadata")[0]?.children || []) {
    fields.push({
      name: node.localName,
      key: node.getAttribute("property") || node.getAttribute("name") || node.localName,
      value: normalizeText(node.textContent || node.getAttribute("content") || ""),
    });
  }
  return fields;
}

export async function parseEpub(file, onProgress = () => {}) {
  const zipLib = requireZip();
  const reader = new zipLib.ZipReader(new zipLib.BlobReader(file));
  const entries = await reader.getEntries();
  const entryMap = new Map(entries.map((entry) => [entry.filename, entry]));
  const names = new Set(entryMap.keys());
  if (!names.has("META-INF/container.xml")) {
    throw new Error("Cannot read EPUB: missing META-INF/container.xml");
  }
  onProgress(`Read ${entries.length} zip entries from ${file.name}`);
  const containerText = await readEntryText(entryMap.get("META-INF/container.xml"));
  const containerDoc = parseXml(containerText, "container.xml");
  const rootfile = firstElement(containerDoc, "rootfile");
  const opfPath = rootfile?.getAttribute("full-path");
  if (!opfPath || !entryMap.has(opfPath)) {
    throw new Error("Cannot read EPUB: OPF path from container.xml does not resolve");
  }
  const opfText = await readEntryText(entryMap.get(opfPath));
  const opfDoc = parseXml(opfText, opfPath);
  const opfDir = opfPath.includes("/") ? opfPath.split("/").slice(0, -1).join("/") : "";
  const manifest = manifestItems(opfDoc, opfDir);
  const cssPaths = manifest.filter((item) => item.mediaType === "text/css" || item.href.endsWith(".css")).map((item) => item.path);
  const xhtmlPaths = manifest.filter((item) => item.mediaType === "application/xhtml+xml" || /\.x?html$/i.test(item.href)).map((item) => item.path);
  const reserved = new Set(["mimetype", "META-INF/container.xml", opfPath, ...cssPaths, ...xhtmlPaths]);
  const navItem = manifest.find((item) => item.properties.split(/\s+/).includes("nav"));
  const ncxItem = manifest.find((item) => item.mediaType === "application/x-dtbncx+xml" || item.id === "ncx");
  if (navItem) reserved.add(navItem.path);
  if (ncxItem) reserved.add(ncxItem.path);
  const resourcePaths = [...names].filter((name) => !reserved.has(name) && !name.endsWith("/"));
  return {
    file,
    reader,
    entries,
    entryMap,
    names,
    opfPath,
    opfText,
    opfDoc,
    manifest,
    spine: spineIdrefs(opfDoc),
    metadata: metadata(opfDoc),
    navPath: navItem?.path || null,
    ncxPath: ncxItem?.path || null,
    cssPaths,
    xhtmlPaths,
    resourcePaths,
    summary: {
      name: file.name,
      size: formatBytes(file.size),
      xhtml: xhtmlPaths.length,
      css: cssPaths.length,
      resources: resourcePaths.length,
    },
  };
}
