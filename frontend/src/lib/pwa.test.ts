import { describe, expect, it } from "vitest";

import { runsInstalled } from "./pwa";

/** A window whose display mode is `mode`, and on iOS `standalone`. */
function windowIn(mode: string, standalone?: boolean): Window {
  return {
    navigator: { standalone },
    matchMedia: (query: string) => ({ matches: query === `(display-mode: ${mode})` }),
  } as unknown as Window;
}

describe("runsInstalled", () => {
  it("knows the installed app by its display mode", () => {
    expect(runsInstalled(windowIn("standalone"))).toBe(true);
    // What the manifest's display_override asks for.
    expect(runsInstalled(windowIn("minimal-ui"))).toBe(true);
  });

  it("knows a home screen app on iOS", () => {
    expect(runsInstalled(windowIn("browser", true))).toBe(true);
  });

  it("is false in a browser tab", () => {
    expect(runsInstalled(windowIn("browser"))).toBe(false);
  });
});
