export function parseXml(text, label = "XML") {
  const doc = new DOMParser().parseFromString(text.replaceAll("&nbsp;", "&#160;"), "application/xml");
  const error = doc.querySelector("parsererror");
  if (error) {
    throw new Error(`${label}: XML parse failed`);
  }
  return doc;
}

export function localName(node) {
  return node?.localName || "";
}

export function elements(doc, name) {
  return [...doc.getElementsByTagName("*")].filter((node) => localName(node) === name);
}

export function firstElement(doc, name) {
  return elements(doc, name)[0] || null;
}

export function normalizeText(text) {
  return (text || "").replace(/\s+/g, " ").trim();
}

export function textWithoutTags(node, ignored = new Set(["rt", "rp", "script", "style"])) {
  const chunks = [];
  const walk = (current) => {
    if (ignored.has(localName(current))) return;
    for (const child of current.childNodes) {
      if (child.nodeType === Node.TEXT_NODE) chunks.push(child.nodeValue);
      if (child.nodeType === Node.ELEMENT_NODE) walk(child);
    }
  };
  walk(node);
  return normalizeText(chunks.join(""));
}
