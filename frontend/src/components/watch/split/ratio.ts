/**
 * How the split view divides its width, as the camera's share in percent. The presets
 * are the old page's menu (web/ts/splitview.ts), in the same order.
 */
export const RATIO_PRESETS = [
  { label: "Presentation only", cam: 0 },
  { label: "Focus presentation", cam: 25 },
  { label: "Split evenly", cam: 50 },
  { label: "Focus camera", cam: 75 },
  { label: "Camera only", cam: 100 },
] as const;

export const DEFAULT_RATIO = 25;

/** Not the old page's key -- it never remembered a ratio -- but namespaced like the SPA's other settings. */
export const RATIO_KEY = "gocast.splitview.camPercent";

export function clampRatio(value: number): number {
  if (!Number.isFinite(value)) return DEFAULT_RATIO;
  return Math.min(100, Math.max(0, Math.round(value)));
}

// localStorage throws in private windows and with blocked site data; a ratio that is
// not remembered is only a nuisance.
export function loadRatio(): number {
  try {
    const stored = window.localStorage.getItem(RATIO_KEY);
    return stored === null ? DEFAULT_RATIO : clampRatio(Number(stored));
  } catch {
    return DEFAULT_RATIO;
  }
}

export function saveRatio(cam: number): void {
  try {
    window.localStorage.setItem(RATIO_KEY, String(clampRatio(cam)));
  } catch {
    // Not remembered.
  }
}

/** The preset the ratio is, if it is exactly one. */
export function presetFor(cam: number): (typeof RATIO_PRESETS)[number] | undefined {
  return RATIO_PRESETS.find((p) => p.cam === cam);
}
