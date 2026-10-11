import { afterEach, describe, expect, it, vi } from "vitest";

import { readSidebarTab, writeSidebarTab } from "./watch-panels-tabs";

afterEach(() => {
  vi.unstubAllGlobals();
  localStorage.clear();
});

describe("sidebar tab memory", () => {
  it("remembers the tab", () => {
    expect(readSidebarTab()).toBeNull();
    writeSidebarTab("sections");
    expect(readSidebarTab()).toBe("sections");
  });

  it("ignores a stored value that is not a tab", () => {
    localStorage.setItem("watch.sidebarTab", "nonsense");
    expect(readSidebarTab()).toBeNull();
  });

  it("survives storage that throws", () => {
    vi.stubGlobal("localStorage", {
      getItem: () => {
        throw new Error("blocked");
      },
      setItem: () => {
        throw new Error("blocked");
      },
    });
    expect(readSidebarTab()).toBeNull();
    expect(() => writeSidebarTab("chat")).not.toThrow();
  });
});
