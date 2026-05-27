function mapById(items) {
  return new Map(items.map((item) => [item.id || item.path, item]));
}

function itemSig(item) {
  return `${item.href}|${item.mediaType}|${item.properties}`;
}

export async function diffStructure(before, after) {
  const bMap = mapById(before.manifest);
  const aMap = mapById(after.manifest);
  const keys = [...new Set([...bMap.keys(), ...aMap.keys()])].sort();
  const manifest = keys.map((key) => {
    const b = bMap.get(key);
    const a = aMap.get(key);
    return {
      id: key,
      before: b ? itemSig(b) : "",
      after: a ? itemSig(a) : "",
      status: !b ? "added" : !a ? "deleted" : itemSig(b) === itemSig(a) ? "same" : "modified",
    };
  });
  const spineChanged = before.spine.join("\n") !== after.spine.join("\n");
  return {
    manifest,
    spine: { before: before.spine, after: after.spine, changed: spineChanged },
    nav: { before: before.navPath, after: after.navPath, changed: before.navPath !== after.navPath },
    ncx: { before: before.ncxPath, after: after.ncxPath, changed: before.ncxPath !== after.ncxPath },
    changed: manifest.some((row) => row.status !== "same") || spineChanged || before.navPath !== after.navPath || before.ncxPath !== after.ncxPath,
  };
}
