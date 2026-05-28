const CORE = ["title", "creator", "identifier", "language"];

function groupMetadata(items) {
  const grouped = new Map();
  for (const item of items) {
    const key = item.name.startsWith("dc:") ? item.name.slice(3) : item.name;
    const stable = item.key || key;
    if (!grouped.has(stable)) grouped.set(stable, []);
    grouped.get(stable).push(item.value);
  }
  return grouped;
}

export function diffMetadata(before, after) {
  const b = groupMetadata(before.metadata);
  const a = groupMetadata(after.metadata);
  const keys = [...new Set([...b.keys(), ...a.keys()])].sort();
  const rows = keys.map((key) => {
    const beforeValue = (b.get(key) || []).join(" | ");
    const afterValue = (a.get(key) || []).join(" | ");
    return { key, before: beforeValue, after: afterValue, changed: beforeValue !== afterValue, core: CORE.includes(key) };
  });
  return {
    rows,
    coreChanged: rows.some((row) => row.core && row.changed),
  };
}
