import { parseXml, localName, textWithoutTags } from "../parsers/xml.js";
import { readEntryText, throwIfAborted } from "../parsers/epub.js";
import { sha256String } from "../util/hash.js";

const BLOCKS = new Set(["p", "h1", "h2", "h3", "h4", "h5", "h6", "li", "td", "blockquote", "pre", "div"]);

function hasBlockDescendant(node) {
  for (const child of node.querySelectorAll("*")) {
    if (BLOCKS.has(localName(child))) return true;
  }
  return false;
}

async function textBlocks(epub, path, options = {}) {
  const { signal } = options;
  const entry = epub.entryMap.get(path);
  if (!entry) return [];
  const doc = parseXml(await readEntryText(entry, { signal }), path);
  const blocks = [];
  for (const node of doc.querySelectorAll("*")) {
    throwIfAborted(signal);
    if (!BLOCKS.has(localName(node)) || hasBlockDescendant(node)) continue;
    const text = textWithoutTags(node);
    if (text) blocks.push({ text, hash: await sha256String(text) });
  }
  return blocks;
}

export async function diffText(before, after, options = {}) {
  const { signal } = options;
  const paths = [...new Set([...before.xhtmlPaths, ...after.xhtmlPaths])].sort();
  const files = [];
  for (const path of paths) {
    throwIfAborted(signal);
    const bExists = before.entryMap.has(path);
    const aExists = after.entryMap.has(path);
    if (!bExists || !aExists) {
      files.push({ path, status: bExists ? "deleted" : "added", before: [], after: [] });
      continue;
    }
    const bBlocks = await textBlocks(before, path, { signal });
    const aBlocks = await textBlocks(after, path, { signal });
    const same = bBlocks.map((b) => b.hash).join("\n") === aBlocks.map((a) => a.hash).join("\n");
    const firstChange = same ? -1 : bBlocks.findIndex((block, index) => block.hash !== aBlocks[index]?.hash);
    files.push({ path, status: same ? "same" : "modified", before: bBlocks, after: aBlocks, firstChange });
  }
  return { files, changed: files.some((file) => file.status !== "same") };
}
