import { parseEpub, throwIfAborted } from "./parsers/epub.js";
import { diffStructure } from "./layers/structure.js";
import { diffText } from "./layers/text.js";
import { diffStyle } from "./layers/style.js";
import { diffResources } from "./layers/resources.js";
import { diffMetadata } from "./layers/metadata.js";
import { renderDiffView } from "./render/diff-view.js";
import { showView, setProgress } from "./render/loading.js";
import { setupTheme } from "./render/theme.js";

const state = {
  beforeFile: null,
  afterFile: null,
  result: null,
  abortController: null,
  objectUrls: [],
};

function $(id) {
  return document.getElementById(id);
}

function updatePicker(slot, file) {
  state[`${slot}File`] = file;
  $(`${slot}-name`).textContent = file ? `${file.name} · ${(file.size / 1024 / 1024).toFixed(2)} MB` : "no file selected · 或拖入此处";
  $("compare-btn").disabled = !(state.beforeFile && state.afterFile);
}

function setupFileInput(slot) {
  const input = $(`${slot}-file`);
  const picker = document.querySelector(`.file-picker[data-slot="${slot}"]`);
  input.addEventListener("change", () => updatePicker(slot, input.files[0] || null));
  for (const event of ["dragenter", "dragover"]) {
    picker.addEventListener(event, (ev) => {
      ev.preventDefault();
      picker.classList.add("drag-over");
    });
  }
  for (const event of ["dragleave", "drop"]) {
    picker.addEventListener(event, (ev) => {
      ev.preventDefault();
      picker.classList.remove("drag-over");
    });
  }
  picker.addEventListener("drop", (ev) => {
    const file = [...ev.dataTransfer.files].find((item) => item.name.toLowerCase().endsWith(".epub"));
    if (file) updatePicker(slot, file);
  });
}

function isAbort(error) {
  return error?.name === "AbortError";
}

function clearObjectUrls() {
  for (const url of state.objectUrls) URL.revokeObjectURL(url);
  state.objectUrls = [];
}

function createObjectUrl(blob) {
  const url = URL.createObjectURL(blob);
  state.objectUrls.push(url);
  return url;
}

async function compare() {
  $("landing-error").textContent = "";
  state.abortController?.abort();
  clearObjectUrls();
  const controller = new AbortController();
  state.abortController = controller;
  const { signal } = controller;
  const isCurrent = () => state.abortController === controller;
  showView("loading");
  try {
    setProgress(5, "Opening before EPUB");
    const before = await parseEpub(state.beforeFile, (msg) => setProgress(10, msg), { signal });
    throwIfAborted(signal);
    setProgress(25, "Opening after EPUB");
    const after = await parseEpub(state.afterFile, (msg) => setProgress(35, msg), { signal });
    throwIfAborted(signal);

    setProgress(45, "Comparing structure");
    const structure = await diffStructure(before, after, { signal });
    throwIfAborted(signal);
    setProgress(55, "Comparing metadata");
    const metadata = diffMetadata(before, after, { signal });
    throwIfAborted(signal);
    setProgress(65, "Comparing text");
    const text = await diffText(before, after, { signal });
    throwIfAborted(signal);
    setProgress(78, "Comparing style");
    const style = await diffStyle(before, after, { signal });
    throwIfAborted(signal);
    setProgress(90, "Hashing resources");
    const resources = await diffResources(before, after, {
      createObjectUrl,
      onProgress: (msg) => setProgress(90, msg),
      signal,
    });
    throwIfAborted(signal);

    state.result = { before, after, layers: { structure, metadata, text, style, resources } };
    renderDiffView(state.result);
    setProgress(100, "Done");
    showView("diff");
  } catch (error) {
    if (!isCurrent()) return;
    clearObjectUrls();
    if (isAbort(error)) {
      $("landing-error").textContent = "Comparison cancelled.";
    } else {
      $("landing-error").textContent = error.message || String(error);
    }
    showView("landing");
  } finally {
    if (isCurrent()) state.abortController = null;
  }
}

function exportJson() {
  if (!state.result) return;
  const resourceRows = state.result.layers.resources.rows.map((row) => {
    const { thumbnails, ...plain } = row;
    return plain;
  });
  const plain = {
    before: state.result.before.summary,
    after: state.result.after.summary,
    layers: {
      ...state.result.layers,
      resources: { ...state.result.layers.resources, rows: resourceRows },
    },
  };
  const blob = new Blob([JSON.stringify(plain, null, 2)], { type: "application/json" });
  const url = URL.createObjectURL(blob);
  const a = document.createElement("a");
  a.href = url;
  a.download = "epub-diff.json";
  a.click();
  URL.revokeObjectURL(url);
}

function setup() {
  setupTheme($("theme-toggle"));
  setupFileInput("before");
  setupFileInput("after");
  $("compare-btn").addEventListener("click", compare);
  $("cancel-btn").addEventListener("click", () => {
    state.abortController?.abort();
    showView("landing");
  });
  $("back-btn").addEventListener("click", () => {
    clearObjectUrls();
    showView("landing");
  });
  $("export-json").addEventListener("click", exportJson);
  $("view-toggle").addEventListener("click", () => {
    document.body.classList.toggle("stacked");
  });
  $("help-toggle").addEventListener("click", () => $("about-modal").showModal());
}

setup();
