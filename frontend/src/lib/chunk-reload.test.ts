import { beforeEach, describe, expect, it, vi } from "vitest";

import { isChunkLoadError, reloadOnce } from "./chunk-reload";

describe("isChunkLoadError", () => {
  it.each([
    "Failed to fetch dynamically imported module: https://x/spa-assets/UsersView-abc.js",
    "error loading dynamically imported module",
    "Importing a module script failed.",
    "Unable to preload CSS for /spa-assets/a.css",
  ])("recognises %s", (message) => {
    expect(isChunkLoadError(new TypeError(message))).toBe(true);
  });

  it("ignores other errors and non-errors", () => {
    expect(isChunkLoadError(new Error("boom"))).toBe(false);
    expect(isChunkLoadError("Failed to fetch dynamically imported module")).toBe(false);
    expect(isChunkLoadError(undefined)).toBe(false);
  });
});

describe("reloadOnce", () => {
  beforeEach(() => sessionStorage.clear());

  it("loads the target the first time", () => {
    const assign = vi.fn();
    expect(reloadOnce("/admin/users", assign, () => 1000)).toBe(true);
    expect(assign).toHaveBeenCalledWith("/admin/users");
  });

  it("does not loop when the reload fails the same way", () => {
    const assign = vi.fn();
    reloadOnce("/admin/users", assign, () => 1000);
    expect(reloadOnce("/admin/users", assign, () => 5000)).toBe(false);
    expect(assign).toHaveBeenCalledTimes(1);
  });

  it("tries again once the window has passed", () => {
    const assign = vi.fn();
    reloadOnce("/a", assign, () => 1000);
    expect(reloadOnce("/b", assign, () => 1000 + 61_000)).toBe(true);
    expect(assign).toHaveBeenLastCalledWith("/b");
  });

  it("does not reload when storage is unavailable", () => {
    const assign = vi.fn();
    vi.stubGlobal("sessionStorage", {
      getItem: () => {
        throw new Error("denied");
      },
    });
    expect(reloadOnce("/a", assign)).toBe(false);
    expect(assign).not.toHaveBeenCalled();
    vi.unstubAllGlobals();
  });
});
