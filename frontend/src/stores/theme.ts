import { defineStore } from "pinia";
import { ref } from "vue";

export type ThemeMode = "light" | "dark" | "system";

/** The theme modes the header offers. */
export const THEME_MODES: { id: ThemeMode; name: string }[] = [
  { id: "light", name: "Light" },
  { id: "dark", name: "Dark" },
  { id: "system", name: "System" },
];

/**
 * Theme selection, kept in the `themeMode` localStorage key. The initial class is applied by the inline script in index.html before first
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
