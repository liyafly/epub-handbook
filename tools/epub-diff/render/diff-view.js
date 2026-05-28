import { formatBytes } from "../util/format.js";
import { renderLineDiff } from "./line-diff.js";

function row(cells) {
  return `<tr>${cells.map((cell) => `<td>${cell}</td>`).join("")}</tr>`;
}

function escapeHtml(value) {
  return String(value).replace(/[&<>"']/g, (char) => ({
    "&": "&amp;",
    "<": "&lt;",
    ">": "&gt;",
    '"': "&quot;",
    "'": "&#39;",
  })[char]);
}

function table(headers, rows) {
  return `<div class="table-wrap"><table><thead><tr>${headers.map((h) => `<th>${h}</th>`).join("")}</tr></thead><tbody>${rows.join("")}</tbody></table></div>`;
}

function counts(rows) {
  return rows.reduce((acc, row) => {
    acc[row.status] = (acc[row.status] || 0) + 1;
    return acc;
  }, {});
}

function renderStructure(layer) {
  const c = counts(layer.manifest);
  return `
    <div class="metric-grid">
      <div class="metric">Manifest changed: ${layer.changed ? "yes" : "no"}</div>
      <div class="metric">Added: ${c.added || 0}</div>
      <div class="metric">Deleted: ${c.deleted || 0}</div>
      <div class="metric">Modified: ${c.modified || 0}</div>
    </div>
    <h3>Spine</h3>
    ${layer.spine.changed ? renderLineDiff(layer.spine.before.join("\n"), layer.spine.after.join("\n")) : "<p class=\"ok\">Spine identical.</p>"}
    <h3>Manifest</h3>
    ${table(["id", "before", "after", "status"], layer.manifest.filter((r) => r.status !== "same").map((r) => row([r.id, r.before, r.after, r.status])))}
  `;
}

function renderText(layer) {
  const changed = layer.files.filter((file) => file.status !== "same");
  if (!changed.length) return `<p class="ok">All ${layer.files.length} XHTML files have identical text content.</p>`;
  return changed.map((file) => {
    if (file.status !== "modified") return `<div class="file-block"><h3>${file.path}</h3><p>${file.status}</p></div>`;
    const index = file.firstChange < 0 ? 0 : file.firstChange;
    const before = file.before[index]?.text || "";
    const after = file.after[index]?.text || "";
    return `<div class="file-block"><h3>${file.path}</h3><p>First changed block: ${index}</p>${renderLineDiff(before, after)}</div>`;
  }).join("");
}

function renderStyle(layer) {
  const summary = layer.summary;
  const changed = layer.files.filter((file) => file.status !== "same");
  return `
    <div class="metric-grid">
      <div class="metric">Added selectors: ${summary.addedSelectors}</div>
      <div class="metric">Deleted selectors: ${summary.deletedSelectors}</div>
      <div class="metric">Modified selectors: ${summary.modifiedSelectors}</div>
    </div>
    ${changed.length ? changed.map((file) => `<div class="file-block"><h3>${file.path}</h3>${renderLineDiff(file.before, file.after)}</div>`).join("") : "<p class=\"ok\">All CSS files identical.</p>"}
  `;
}

function renderResources(layer) {
  const c = counts(layer.rows);
  const changed = layer.rows.filter((resource) => resource.status !== "same");
  const item = (resource) => {
    const details = `${resource.status}: ${resource.path} (${formatBytes(resource.before?.size || 0)} -> ${formatBytes(resource.after?.size || 0)})`;
    const thumbs = resource.thumbnails ? `
      <div class="resource-thumbs">
        <figure>
          <img src="${escapeHtml(resource.thumbnails.before.url)}" alt="before ${escapeHtml(resource.path)}">
          <figcaption>before</figcaption>
        </figure>
        <figure>
          <img src="${escapeHtml(resource.thumbnails.after.url)}" alt="after ${escapeHtml(resource.path)}">
          <figcaption>after</figcaption>
        </figure>
      </div>
    ` : "";
    return `<li><div>${escapeHtml(details)}</div>${thumbs}</li>`;
  };
  return `
    <div class="metric-grid">
      <div class="metric">Added: ${c.added || 0}</div>
      <div class="metric">Deleted: ${c.deleted || 0}</div>
      <div class="metric">Modified: ${c.modified || 0}</div>
      <div class="metric">Unchanged: ${c.same || 0}</div>
    </div>
    <ul class="resource-list">
      ${changed.map(item).join("") || "<li class=\"ok\">All resources identical.</li>"}
    </ul>
  `;
}

function renderMetadata(layer) {
  return table(["field", "before", "after", "status"], layer.rows.map((item) => row([
    item.core ? `<strong>${item.key}</strong>` : item.key,
    item.before || "<em>empty</em>",
    item.after || "<em>empty</em>",
    item.changed ? "<span class=\"changed\">changed</span>" : "<span class=\"ok\">same</span>",
  ])));
}

const RENDERERS = {
  structure: renderStructure,
  text: renderText,
  style: renderStyle,
  resources: renderResources,
  metadata: renderMetadata,
};

export function renderDiffView(result) {
  const before = result.before.summary;
  const after = result.after.summary;
  const status = [
    result.layers.metadata.coreChanged ? "Core metadata differs" : "Core metadata identical",
    result.layers.text.changed ? "Text content differs" : "Text content identical",
    result.layers.structure.spine.changed ? "Spine differs" : "Spine identical",
  ];
  document.getElementById("info-bar").innerHTML = `
    <div>before: ${before.name} (${before.size} · ${before.xhtml} XHTML · ${before.css} CSS · ${before.resources} resources)</div>
    <div>after: ${after.name} (${after.size} · ${after.xhtml} XHTML · ${after.css} CSS · ${after.resources} resources)</div>
    <div class="status-line">${status.join(" · ")}</div>
  `;

  const nav = document.getElementById("layers-nav");
  nav.innerHTML = "";
  const main = document.getElementById("layers-main");
  main.innerHTML = "";
  for (const [key, title] of [["structure", "Structure"], ["text", "Text"], ["style", "Style"], ["resources", "Resources"], ["metadata", "Metadata"]]) {
    const anchor = document.createElement("a");
    anchor.href = `#layer-${key}`;
    anchor.textContent = title;
    nav.append(anchor);
    const section = document.createElement("details");
    section.className = "layer";
    section.id = `layer-${key}`;
    section.open = true;
    section.innerHTML = `<summary>${title}</summary><div class="layer-body">${RENDERERS[key](result.layers[key])}</div>`;
    main.append(section);
  }
}
