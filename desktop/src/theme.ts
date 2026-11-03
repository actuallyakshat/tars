// Light or dark. "system" follows the macOS appearance live; light and dark
// pin it. The resolved theme lands on <html data-theme>, which is what the
// stylesheet's token blocks key off, and on the native window so the title
// bar and traffic lights match.
import { getCurrentWindow } from "@tauri-apps/api/window";

export type ThemePref = "system" | "light" | "dark";

export const THEME_KEY = "tars.theme";

export const THEMES: { id: ThemePref; label: string }[] = [
  { id: "system", label: "System" },
  { id: "light", label: "Light" },
  { id: "dark", label: "Dark" },
];

export function storedTheme(): ThemePref {
  const v = localStorage.getItem(THEME_KEY);
  return v === "light" || v === "dark" ? v : "system";
}

// jsdom has no matchMedia, so tests get the light theme.
const darkQuery = typeof window.matchMedia === "function" ? window.matchMedia("(prefers-color-scheme: dark)") : null;

let pref: ThemePref = "system";

function paint() {
  const dark = pref === "dark" || (pref === "system" && !!darkQuery?.matches);
  document.documentElement.dataset.theme = dark ? "dark" : "light";
}

darkQuery?.addEventListener("change", () => {
  if (pref === "system") paint();
});

export function applyTheme(next: ThemePref) {
  pref = next;
  paint();
  // null hands the window back to the system appearance. Outside the tars
  // window (tests) there is no native window to tell.
  try {
    void getCurrentWindow().setTheme(next === "system" ? null : next).catch(() => {});
  } catch {
    /* not in Tauri */
  }
}
