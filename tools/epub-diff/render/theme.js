const ORDER = ["auto", "light", "dark"];

export function setupTheme(button) {
  let mode = localStorage.getItem("epub-diff-theme") || "auto";
  const apply = () => {
    document.documentElement.dataset.theme = mode === "auto" ? "" : mode;
    button.textContent = `Theme: ${mode}`;
    localStorage.setItem("epub-diff-theme", mode);
  };
  button.addEventListener("click", () => {
    mode = ORDER[(ORDER.indexOf(mode) + 1) % ORDER.length];
    apply();
  });
  apply();
}
