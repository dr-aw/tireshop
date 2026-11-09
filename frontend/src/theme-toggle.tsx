import { useEffect, useState } from "react";
import "./theme.css";

type Theme = "light" | "dark";
const key = "tireshop.theme";
function saved(): Theme | null {
  try {
    const value = localStorage.getItem(key);
    return value === "light" || value === "dark" ? value : null;
  } catch {
    return null;
  }
}
export function ThemeToggle() {
  const [preference, setPreference] = useState<Theme | null>(null);
  const [systemDark, setSystemDark] = useState(false);
  const [initialized, setInitialized] = useState(false);
  const theme = preference ?? (systemDark ? "dark" : "light");
  useEffect(() => {
    const media = matchMedia("(prefers-color-scheme: dark)");
    setPreference(saved());
    setInitialized(true);
    setSystemDark(media.matches);
    const update = () => setSystemDark(media.matches);
    const storage = (event: StorageEvent) => {
      if (event.key === key || event.key === null) setPreference(saved());
    };
    media.addEventListener("change", update);
    window.addEventListener("storage", storage);
    return () => {
      media.removeEventListener("change", update);
      window.removeEventListener("storage", storage);
    };
  }, []);
  useEffect(() => {
    if (initialized) document.documentElement.dataset.theme = theme;
  }, [theme, initialized]);
  const label =
    theme === "dark" ? "Включить светлую тему" : "Включить тёмную тему";
  return (
    <button
      type="button"
      className="theme-toggle"
      aria-label={label}
      title={label}
      onClick={() => {
        const next = theme === "dark" ? "light" : "dark";
        setPreference(next);
        try {
          localStorage.setItem(key, next);
        } catch {}
      }}
    >
      <svg
        width="22"
        height="22"
        viewBox="0 0 24 24"
        fill="none"
        stroke="currentColor"
        strokeWidth="1.7"
        strokeLinecap="round"
        aria-hidden="true"
      >
        {theme === "dark" ? (
          <>
            <circle cx="12" cy="12" r="4" />
            <path d="M12 2v2m0 16v2M2 12h2m16 0h2M5 5l1.5 1.5m11 11L19 19M5 19l1.5-1.5m11-11L19 5" />
          </>
        ) : (
          <path d="M20.5 13A8.5 8.5 0 0 1 11 3.5 8.5 8.5 0 1 0 20.5 13Z" />
        )}
      </svg>
    </button>
  );
}
