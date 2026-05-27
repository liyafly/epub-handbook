function escapeHtml(value) {
  return value.replace(/[&<>"']/g, (char) => ({
    "&": "&amp;",
    "<": "&lt;",
    ">": "&gt;",
    '"': "&quot;",
    "'": "&#39;",
  })[char]);
}

export function renderLineDiff(beforeText, afterText) {
  const before = beforeText.split(/\r?\n/);
  const after = afterText.split(/\r?\n/);
  const rows = [];
  const max = Math.max(before.length, after.length);
  for (let index = 0; index < max; index += 1) {
    const b = before[index] ?? "";
    const a = after[index] ?? "";
    if (b === a) {
      rows.push(`<tr class="line-same"><td>${index + 1}</td><td>${escapeHtml(b)}</td><td>${index + 1}</td><td>${escapeHtml(a)}</td></tr>`);
    } else {
      rows.push(`<tr class="line-del"><td>${index + 1}</td><td>${escapeHtml(b)}</td><td></td><td></td></tr>`);
      rows.push(`<tr class="line-add"><td></td><td></td><td>${index + 1}</td><td>${escapeHtml(a)}</td></tr>`);
    }
  }
  return `<table class="diff-table"><tbody>${rows.join("")}</tbody></table>`;
}
