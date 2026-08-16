import { defineStore } from "pinia";
import { ref } from "vue";

export type ThemeMode = "light" | "dark" | "system";

/** Mode list and labels as defined by the Alpine theme store in web/assets/init.js. */
export const THEME_MODES: { id: ThemeMode; name: string }[] = [
  { id: "light", name: "Light" },
  { id: "dark", name: "Dark" },
  { id: "system", name: "System" },
];

/**
 * Theme selection, sharing the `themeMode` localStorage key with web/assets/init.js so
 * a choice made on a migrated page is honoured by the server-rendered ones and vice
 * versa. The initial class is applied by the inline script in index.html before first
 * paint; this store handles later changes.
 */
export const useThemeStore = defineStore("theme", () => {
  const mode = ref<ThemeMode>((localStorage.themeMode as ThemeMode) ?? "system");
  const prefersDark = window.matchMedia("(prefers-color-scheme: dark)");

  function apply(): void {
    const dark = mode.value === "dark" || (mode.value === "system" && prefersDark.matches);
    document.documentElement.classList.toggle("dark", dark);
  }

  function setMode(next: ThemeMode): void {
    mode.value = next;
    localStorage.themeMode = next;
    apply();
  }

  prefersDark.addEventListener("change", apply);

  return { mode, modes: THEME_MODES, setMode };
});
