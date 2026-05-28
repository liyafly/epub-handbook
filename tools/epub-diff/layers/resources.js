import { readEntryBlob, readEntryHash, throwIfAborted } from "../parsers/epub.js";

function typeOf(path) {
  const ext = path.split(".").pop()?.toLowerCase() || "";
  if (["png", "jpg", "jpeg", "gif", "svg", "webp", "avif"].includes(ext)) return "image";
  if (["otf", "ttf", "woff", "woff2"].includes(ext)) return "font";
  if (["mp3", "m4a", "ogg"].includes(ext)) return "audio";
  return "other";
}

function imageMime(path) {
  const ext = path.split(".").pop()?.toLowerCase() || "";
  if (ext === "jpg") return "image/jpeg";
  if (ext === "svg") return "image/svg+xml";
  if (["png", "jpeg", "gif", "webp", "avif"].includes(ext)) return `image/${ext}`;
  return "application/octet-stream";
}

async function entryInfo(epub, path, options = {}) {
  const { signal } = options;
  const entry = epub.entryMap.get(path);
  if (!entry) return null;
  const { sha256, size } = await readEntryHash(entry, { signal });
  return { path, size: entry.uncompressedSize ?? size, sha256, type: typeOf(path) };
}

async function imageThumb(epub, path, options = {}) {
  const { createObjectUrl, signal } = options;
  const entry = epub.entryMap.get(path);
  if (!entry) return null;
  const blob = await readEntryBlob(entry, imageMime(path), { signal });
  throwIfAborted(signal);
  const url = createObjectUrl ? createObjectUrl(blob) : URL.createObjectURL(blob);
  throwIfAborted(signal);
  return { url, type: imageMime(path), size: entry.uncompressedSize ?? blob.size };
}

export async function diffResources(before, after, options = {}) {
  const { createObjectUrl, onProgress = () => {}, signal } = options;
  const paths = [...new Set([...before.resourcePaths, ...after.resourcePaths])].sort();
  const rows = [];
  for (const [index, path] of paths.entries()) {
    throwIfAborted(signal);
    onProgress(`Hashing resource ${index + 1}/${paths.length}: ${path}`);
    const b = await entryInfo(before, path, { signal });
    const a = await entryInfo(after, path, { signal });
    const status = !b ? "added" : !a ? "deleted" : b.sha256 === a.sha256 ? "same" : "modified";
    const row = { path, before: b, after: a, status, type: (b || a).type };
    if (status === "modified" && row.type === "image" && b && a) {
      row.thumbnails = {
        before: await imageThumb(before, path, { createObjectUrl, signal }),
        after: await imageThumb(after, path, { createObjectUrl, signal }),
      };
    }
    rows.push(row);
  }
  return { rows, changed: rows.some((row) => row.status !== "same") };
}
