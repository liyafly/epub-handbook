import { parseEpub } from "./parsers/epub.js";
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
  cancelled: false,
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

function throwIfCancelled() {
  if (state.cancelled) throw new Error("Cancelled");
}

async function compare() {
  $("landing-error").textContent = "";
  state.cancelled = false;
  showView("loading");
  try {
    setProgress(5, "Opening before EPUB");
    const before = await parseEpub(state.beforeFile, (msg) => setProgress(10, msg));
    throwIfCancelled();
    setProgress(25, "Opening after EPUB");
    const after = await parseEpub(state.afterFile, (msg) => setProgress(35, msg));
    throwIfCancelled();

    setProgress(45, "Comparing structure");
    const structure = await diffStructure(before, after);
    throwIfCancelled();
    setProgress(55, "Comparing metadata");
    const metadata = diffMetadata(before, after);
    throwIfCancelled();
    setProgress(65, "Comparing text");
    const text = await diffText(before, after);
    throwIfCancelled();
    setProgress(78, "Comparing style");
    const style = await diffStyle(before, after);
    throwIfCancelled();
    setProgress(90, "Hashing resources");
    const resources = await diffResources(before, after);

    state.result = { before, after, layers: { structure, metadata, text, style, resources } };
    renderDiffView(state.result);
    setProgress(100, "Done");
    showView("diff");
  } catch (error) {
    showView("landing");
    $("landing-error").textContent = error.message || String(error);
  }
}

function exportJson() {
  if (!state.result) return;
  const plain = {
    before: state.result.before.summary,
    after: state.result.after.summary,
    layers: state.result.layers,
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
    state.cancelled = true;
    showView("landing");
  });
  $("back-btn").addEventListener("click", () => showView("landing"));
  $("export-json").addEventListener("click", exportJson);
  $("view-toggle").addEventListener("click", () => {
    document.body.classList.toggle("stacked");
  });
  $("help-toggle").addEventListener("click", () => $("about-modal").showModal());
}

setup();
