import { describe, expect, it } from "vitest";
import type { RouteLocationNormalized } from "vue-router";

import { legacyStartPageRedirect, router } from "@/router";

/**
 * The server-rendered start page was one path with a `view` query parameter. Those
 * URLs are in bookmarks, in browser history and in links sent between students, and
 * an untranslated one lands on the start page showing the wrong thing rather than
 * failing — which is why this is tested rather than eyeballed.
 */
function location(path: string, query: Record<string, string> = {}): RouteLocationNormalized {
  return { path, query } as unknown as RouteLocationNormalized;
}

describe("legacyStartPageRedirect", () => {
  it("leaves a URL this router already uses alone", () => {
    expect(legacyStartPageRedirect(location("/"))).toBeNull();
    expect(legacyStartPageRedirect(location("/", { year: "2026", term: "W" }))).toBeNull();
  });

  it("ignores paths that were never the start page", () => {
    expect(legacyStartPageRedirect(location("/settings", { view: "3" }))).toBeNull();
    expect(legacyStartPageRedirect(location("/course/2026/W/eidi"))).toBeNull();
  });

  it.each([
    ["the course list", "1", "my-courses"],
    ["the public course list", "2", "public-courses"],
  ])("translates %s and keeps the semester", (_name, view, expected) => {
    expect(legacyStartPageRedirect(location("/", { view, year: "2026", term: "W" }))).toEqual({
      name: expected,
      query: { year: "2026", term: "W" },
    });
  });

  it("translates a course into the path form the server already redirects to", () => {
    expect(
      legacyStartPageRedirect(location("/", { view: "3", year: "2026", term: "W", slug: "eidi" })),
    ).toEqual({ name: "course", params: { year: "2026", term: "W", slug: "eidi" } });
  });

  it("strips the leftovers of an explicit main view", () => {
    // `?view=0&slug=eidi` is the start page with a slug the old state machine ignored.
    expect(legacyStartPageRedirect(location("/", { view: "0", slug: "eidi" }))).toEqual({
      name: "home",
      query: {},
    });
  });

  it.each([
    ["no slug", { view: "3", year: "2026", term: "W" }],
    ["no semester", { view: "3", slug: "eidi" }],
    ["half a semester", { view: "3", slug: "eidi", year: "2026" }],
  ])("falls back to the start page for a course URL with %s", (_name, query) => {
    // The course route cannot be built from these. Nothing generated them, so this is
    // about not stranding a hand-edited URL on a blank page.
    expect(legacyStartPageRedirect(location("/", query))).toEqual({ name: "home" });
  });

  it("treats a repeated parameter as absent rather than picking one", () => {
    const repeated = { path: "/", query: { view: "3", slug: ["a", "b"], year: "2026", term: "W" } };
    expect(legacyStartPageRedirect(repeated as unknown as RouteLocationNormalized)).toEqual({
      name: "home",
    });
  });
});

describe("router", () => {
  it("resolves a translated legacy URL through the navigation guard", async () => {
    // Not the pure function this time: that it is actually wired into the router.
    await router.push("/?view=3&year=2026&term=W&slug=eidi");

    expect(router.currentRoute.value.name).toBe("course");
    expect(router.currentRoute.value.params).toEqual({ year: "2026", term: "W", slug: "eidi" });
  });

  it("routes the semester path into the query form", async () => {
    await router.push("/semester/2025/S");

    expect(router.currentRoute.value.name).toBe("home");
    expect(router.currentRoute.value.query).toEqual({ year: "2025", term: "S" });
  });
});
