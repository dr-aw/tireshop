// Apply before rendering to avoid a flash of the wrong theme.
(() => {
  let theme;
  try {
    theme = localStorage.getItem("tireshop.theme");
  } catch {}
  document.documentElement.dataset.theme =
    theme === "light" || theme === "dark"
      ? theme
      : matchMedia("(prefers-color-scheme: dark)").matches
        ? "dark"
        : "light";
})();
