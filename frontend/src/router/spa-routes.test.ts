import { readFileSync } from "node:fs";
import { resolve } from "node:path";

import { describe, expect, it } from "vitest";

import { router } from "@/router";

/**
 * A path in `spaRoutes` but not in this router does not fail politely: Go serves the
 * shell, the guard hands the path back, and the browser reloads forever.
 *
 * The two path syntaxes agree, so the tables are compared as literal strings — which
 * also catches a parameter renamed on one side only.
 */

/** Resolved from the working directory, which vitest sets to `frontend/`. */
const routerGo = resolve(process.cwd(), "../web/router.go");

/** The keys of the `spaRoutes` map literal in web/router.go. */
function spaRoutesFromGo(): string[] {
  const source = readFileSync(routerGo, "utf8");

  const start = source.indexOf("var spaRoutes = map[string]bool{");
  expect(start, "could not find `var spaRoutes` in web/router.go").toBeGreaterThan(-1);
  const end = source.indexOf("\n}", start);
  expect(end, "the spaRoutes literal in web/router.go is not terminated").toBeGreaterThan(-1);

  const body = source.slice(start, end);
  return [...body.matchAll(/"([^"]+)":\s*true/g)].map((match) => match[1]);
}

describe("the routes Go serves the SPA shell for", () => {
  const paths = spaRoutesFromGo();

  // Otherwise the suite passes when the parse above quietly stops matching.
  it("are found in web/router.go", () => {
    expect(paths.length).toBeGreaterThan(0);
    expect(paths).toContain("/settings");
  });

  it.each(paths)("%s is claimed by this router", (path) => {
    const known = router.getRoutes().map((route) => route.path);
    expect(
      known,
      `web/router.go serves the SPA shell for ${path}, but no route here matches it, ` +
        `so the browser would reload it forever. Add it to routes in src/router/index.ts.`,
    ).toContain(path);
  });
});
