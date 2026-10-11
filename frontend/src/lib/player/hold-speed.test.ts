import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

import { fakePlayer } from "@/test/fake-player";

import { attachHoldSpeed } from "./hold-speed";

beforeEach(() => vi.useFakeTimers());
afterEach(() => vi.useRealTimers());

const mouse = (type: string, init: MouseEventInit = {}) => new MouseEvent(type, { bubbles: true, cancelable: true, ...init });

describe("attachHoldSpeed", () => {
  it("plays at 2x while held on the picture and restores the rate on release", () => {
    const { player, state, el } = fakePlayer();
    state.rate = 1.5;
    const hold = attachHoldSpeed(player);
    el.dispatchEvent(mouse("mousedown"));
    vi.advanceTimersByTime(199);
    expect(state.rate).toBe(1.5);
    vi.advanceTimersByTime(1);
    expect(state.rate).toBe(2);
    expect(hold.active()).toBe(true);
    document.dispatchEvent(mouse("mouseup"));
    expect(state.rate).toBe(1.5);
    expect(hold.active()).toBe(false);
    hold.dispose();
  });

  it("does nothing for a short press", () => {
    const { player, state, el } = fakePlayer();
    const hold = attachHoldSpeed(player);
    el.dispatchEvent(mouse("mousedown"));
    vi.advanceTimersByTime(100);
    document.dispatchEvent(mouse("mouseup"));
    vi.advanceTimersByTime(500);
    expect(state.rate).toBe(1);
    hold.dispose();
  });

  it("ignores presses on the controls and other buttons than the main one", () => {
    const { player, state, el } = fakePlayer();
    const bar = document.createElement("div");
    bar.className = "vjs-control-bar";
    const button = document.createElement("button");
    bar.appendChild(button);
    el.appendChild(bar);
    const hold = attachHoldSpeed(player);
    button.dispatchEvent(mouse("mousedown"));
    el.dispatchEvent(mouse("mousedown", { button: 2 }));
    vi.advanceTimersByTime(1000);
    expect(state.rate).toBe(1);
    hold.dispose();
  });

  it("swallows the click that ends a hold, so the video does not pause", () => {
    const { player, el } = fakePlayer();
    const hold = attachHoldSpeed(player);
    const seen = vi.fn();
    el.addEventListener("click", seen);
    el.dispatchEvent(mouse("mousedown"));
    vi.advanceTimersByTime(300);
    document.dispatchEvent(mouse("mouseup"));
    el.dispatchEvent(mouse("click"));
    expect(seen).not.toHaveBeenCalled();
    vi.advanceTimersByTime(60);
    el.dispatchEvent(mouse("click"));
    expect(seen).toHaveBeenCalledOnce();
    hold.dispose();
  });

  it("restores the rate and removes its listeners on dispose mid-hold", () => {
    const { player, state, el } = fakePlayer();
    const hold = attachHoldSpeed(player);
    el.dispatchEvent(mouse("mousedown"));
    vi.advanceTimersByTime(300);
    hold.dispose();
    expect(state.rate).toBe(1);
    el.dispatchEvent(mouse("mousedown"));
    vi.advanceTimersByTime(300);
    expect(state.rate).toBe(1);
  });
});
