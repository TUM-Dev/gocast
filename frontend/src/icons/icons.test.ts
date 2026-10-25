import { readFileSync } from "node:fs";

import { describe, expect, it } from "vitest";

import { buildCss } from "../../scripts/build-icons.mjs";

// Read from disk: vitest does not hand out processed CSS as a string.
const css = readFileSync("src/styles/fa-icons.css", "utf8") // vitest runs from frontend/;
const sources = import.meta.glob<string>("../**/*.{vue,ts}", { query: "?raw", import: "default", eager: true });

// Font Awesome classes that style or size an icon instead of naming one.
const MODIFIERS = new Set(["solid", "regular", "brands", "spin"]);

function usedIcons(): Set<string> {
  const used = new Set<string>();
  for (const [file, text] of Object.entries(sources)) {
    if (file.includes("/gen/") || file.endsWith(".test.ts")) continue;
    for (const m of text.matchAll(/(?<![\w./-])fa-([a-z0-9]+(?:-[a-z0-9]+)*)/g)) {
      if (!MODIFIERS.has(m[1])) used.add(m[1]);
    }
  }
  return used;
}

describe("icon stylesheet", () => {
  // An icon class with no rule renders as a solid square, with no error anywhere.
  it("defines every fa-* class the app uses", () => {
    const missing = [...usedIcons()].filter((name) => !new RegExp(`\\.fa-${name}\\b(?!-)`).test(css));
    expect(missing, "add these to src/icons/icons.json (and the SVG), then run `pnpm icons`").toEqual([]);
  });

  it("is what scripts/build-icons.mjs generates", () => {
    expect(css, "run `pnpm icons`").toBe(buildCss());
  });
});
