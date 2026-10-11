import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

import { createOverlayIcon } from "./overlay-icon";

beforeEach(() => vi.useFakeTimers());
afterEach(() => vi.useRealTimers());

describe("createOverlayIcon", () => {
  it("shows one icon, replaces it, and removes it after the timeout", () => {
    const host = document.createElement("div");
    const overlay = createOverlayIcon(host, 1000);
    const container = host.querySelector(".vjs-overlay-icon-container")!;
    overlay.show("vjs-icon-play");
    overlay.show("fa-solid fa-forward");
    expect(container.querySelectorAll("i")).toHaveLength(1);
    expect(container.querySelector("i")!.className).toBe("fa-solid fa-forward");
    expect(container.classList.contains("vjs-overlay-icon-animate")).toBe(true);
    vi.advanceTimersByTime(1000);
    expect(container.querySelector("i")).toBeNull();
    overlay.dispose();
    expect(host.children).toHaveLength(0);
  });
});
