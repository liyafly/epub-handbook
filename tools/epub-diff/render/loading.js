export function showView(id) {
  for (const view of document.querySelectorAll(".view")) {
    view.classList.toggle("active", view.id === id);
  }
}

export function setProgress(value, text) {
  document.getElementById("progress").value = value;
  document.getElementById("loading-status").textContent = text || "";
}
