import { readEntryBuffer } from "../parsers/epub.js";
import { sha256Buffer } from "../util/hash.js";

function typeOf(path) {
  const ext = path.split(".").pop()?.toLowerCase() || "";
  if (["png", "jpg", "jpeg", "gif", "svg", "webp", "avif"].includes(ext)) return "image";
  if (["otf", "ttf", "woff", "woff2"].includes(ext)) return "font";
  if (["mp3", "m4a", "ogg"].includes(ext)) return "audio";
  return "other";
}

async function entryInfo(epub, path) {
  const entry = epub.entryMap.get(path);
  if (!entry) return null;
  const buffer = await readEntryBuffer(entry);
  return { path, size: entry.uncompressedSize || buffer.byteLength, sha256: await sha256Buffer(buffer), type: typeOf(path) };
}

export async function diffResources(before, after) {
  const paths = [...new Set([...before.resourcePaths, ...after.resourcePaths])].sort();
  const rows = [];
  for (const path of paths) {
    const b = await entryInfo(before, path);
    const a = await entryInfo(after, path);
    const status = !b ? "added" : !a ? "deleted" : b.sha256 === a.sha256 ? "same" : "modified";
    rows.push({ path, before: b, after: a, status, type: (b || a).type });
  }
  return { rows, changed: rows.some((row) => row.status !== "same") };
}
