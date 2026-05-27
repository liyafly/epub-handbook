import { readEntryText, throwIfAborted } from "../parsers/epub.js";

function parseSelectors(css) {
  const clean = css.replace(/\/\*[\s\S]*?\*\//g, "");
  const map = new Map();
  for (const match of clean.matchAll(/([^{}]+)\{([^{}]*)\}/g)) {
    const selector = match[1].trim();
    const body = match[2].replace(/\s+/g, " ").trim();
    if (selector) map.set(selector, body);
  }
  return map;
}

export async function diffStyle(before, after, options = {}) {
  const { signal } = options;
  const paths = [...new Set([...before.cssPaths, ...after.cssPaths])].sort();
  const files = [];
  let addedSelectors = 0;
  let deletedSelectors = 0;
  let modifiedSelectors = 0;
  for (const path of paths) {
    throwIfAborted(signal);
    const bText = before.entryMap.has(path) ? await readEntryText(before.entryMap.get(path), { signal }) : "";
    const aText = after.entryMap.has(path) ? await readEntryText(after.entryMap.get(path), { signal }) : "";
    const bSel = parseSelectors(bText);
    const aSel = parseSelectors(aText);
    const selectors = [...new Set([...bSel.keys(), ...aSel.keys()])];
    const changes = selectors.map((selector) => {
      const b = bSel.get(selector);
      const a = aSel.get(selector);
      const status = b === undefined ? "added" : a === undefined ? "deleted" : b === a ? "same" : "modified";
      if (status === "added") addedSelectors += 1;
      if (status === "deleted") deletedSelectors += 1;
      if (status === "modified") modifiedSelectors += 1;
      return { selector, status, before: b || "", after: a || "" };
    });
    files.push({
      path,
      before: bText,
      after: aText,
      status: !bText ? "added" : !aText ? "deleted" : bText === aText ? "same" : "modified",
      changes,
    });
  }
  return {
    files,
    summary: { addedSelectors, deletedSelectors, modifiedSelectors },
    changed: files.some((file) => file.status !== "same"),
  };
}
