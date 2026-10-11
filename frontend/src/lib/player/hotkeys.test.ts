import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

import { fakePlayer } from "@/test/fake-player";
import { timeRanges } from "./types";

import { attachHotkeys, bookmarkHotkey, defaultHotkeys, HOTKEY_HELP, isTypingTarget, timeAtFraction } from "./hotkeys";

let detach: (() => void) | null = null;
afterEach(() => {
  detach?.();
  detach = null;
  document.body.replaceChildren();
});
beforeEach(() => document.body.replaceChildren());

function press(target: EventTarget, key: string, init: KeyboardEventInit = {}) {
  const event = new KeyboardEvent("keydown", { key, bubbles: true, cancelable: true, ...init });
  target.dispatchEvent(event);
  return event;
}

function setup(scope: "player" | "page" = "player") {
  const f = fakePlayer({ duration: 200 });
  document.body.appendChild(f.el);
  detach = attachHotkeys(f.player, { scope, seekSeconds: 10 });
  return f;
}

describe("hotkeys", () => {
  it("toggles play and pause on k and space, and swallows the event", () => {
    const { state, el } = setup();
    const e = press(el, "k");
    expect(state.paused).toBe(false);
    expect(e.defaultPrevented).toBe(true);
    press(el, " ");
    expect(state.paused).toBe(true);
  });

  it("seeks by the skip distance and clamps to the media", () => {
    const { state, el } = setup();
    state.time = 5;
    press(el, "j");
    expect(state.time).toBe(0);
    press(el, "ArrowRight");
    press(el, "l");
    expect(state.time).toBe(20);
    state.time = 195;
    press(el, "l");
    expect(state.time).toBe(200);
  });

  it("seeks to a tenth of the length on digits, and to the ends", () => {
    const { state, el } = setup();
    press(el, "7");
    expect(state.time).toBe(140);
    press(el, "End");
    expect(state.time).toBe(200);
    press(el, "Home");
    expect(state.time).toBe(0);
  });

  it("changes volume in steps, unmuting", () => {
    const { state, el } = setup();
    state.volume = 0.5;
    state.muted = true;
    press(el, "ArrowDown");
    expect(state.volume).toBeCloseTo(0.45);
    expect(state.muted).toBe(false);
    state.volume = 1;
    press(el, "ArrowUp");
    expect(state.volume).toBe(1);
  });

  it("mutes, toggles fullscreen and steps the rate (> faster, < slower)", () => {
    const { state, el } = setup();
    press(el, "m");
    expect(state.muted).toBe(true);
    press(el, "f");
    expect(state.fullscreen).toBe(true);
    press(el, "f");
    expect(state.fullscreen).toBe(false);
    press(el, ">");
    expect(state.rate).toBe(1.5);
    press(el, "<");
    press(el, "<");
    expect(state.rate).toBe(0.5);
    press(el, "<");
    expect(state.rate).toBe(0.5);
  });

  it("flashes an overlay icon for the action", () => {
    const { el } = setup();
    press(el, "k");
    expect(el.querySelector(".vjs-overlay-icon-wrapper i")?.className).toBe("vjs-icon-play");
  });

  it("leaves keys with modifiers, unknown keys and typing alone", () => {
    const { state, el } = setup("page");
    expect(press(el, "k", { ctrlKey: true }).defaultPrevented).toBe(false);
    expect(press(el, "x").defaultPrevented).toBe(false);
    const input = document.createElement("input");
    document.body.appendChild(input);
    input.focus();
    expect(press(input, "k").defaultPrevented).toBe(false);
    expect(state.paused).toBe(true);
  });

  it("player scope ignores keys elsewhere, page scope hears them", () => {
    const player = setup("player");
    press(document.body, "k");
    expect(player.state.paused).toBe(true);
    detach?.();
    const page = setup("page");
    press(document.body, "k");
    expect(page.state.paused).toBe(false);
  });

  it("does not take arrow keys from an open menu", () => {
    const { state, el } = setup();
    const menu = document.createElement("div");
    menu.className = "vjs-menu";
    el.appendChild(menu);
    state.time = 50;
    press(menu, "ArrowRight");
    expect(state.time).toBe(50);
  });

  it("takes extra actions, and a replacement by name", () => {
    const f = fakePlayer();
    document.body.appendChild(f.el);
    let hit = "";
    detach = attachHotkeys(f.player, {
      scope: "player",
      seekSeconds: 10,
      extra: {
        skip: { keys: ["s"], action: "", label: "", handle: () => void (hit = "skip") },
        mute: { keys: ["m"], action: "", label: "", handle: () => void (hit = "mute") },
      },
    });
    press(f.el, "s");
    press(f.el, "m");
    expect(hit).toBe("mute");
    expect(f.state.muted).toBe(false);
  });

  it("removes its listener and icon on detach", () => {
    const { state, el } = setup();
    detach?.();
    detach = null;
    press(el, "k");
    expect(state.paused).toBe(true);
    expect(el.querySelector(".vjs-overlay-icon-container")).toBeNull();
  });
});

describe("helpers", () => {
  it("finds the time a fraction into possibly several ranges", () => {
    expect(timeAtFraction(timeRanges([0, 100]), 0.25)).toBe(25);
    expect(timeAtFraction(timeRanges([0, 50], [60, 110]), 0.75)).toBe(85);
    expect(timeAtFraction(timeRanges(), 0.5)).toBeUndefined();
  });

  it("recognises typing targets", () => {
    const text = document.createElement("input");
    const box = document.createElement("input");
    box.type = "checkbox";
    expect(isTypingTarget(text)).toBe(true);
    expect(isTypingTarget(box)).toBe(false);
    expect(isTypingTarget(document.createElement("textarea"))).toBe(true);
    expect(isTypingTarget(document.createElement("button"))).toBe(false);
    expect(isTypingTarget(null)).toBe(false);
  });

  it("documents every binding for the shortcuts popup", () => {
    expect(HOTKEY_HELP).toHaveLength(Object.keys(defaultHotkeys(10)).length);
    expect(HOTKEY_HELP).toContainEqual({ action: "Increase playback rate", keys: ">" });
  });
});

describe("the bookmark key", () => {
  function withBookmarkKey(onAdd: () => void) {
    const f = fakePlayer({ duration: 200 });
    document.body.appendChild(f.el);
    detach = attachHotkeys(f.player, { scope: "page", seekSeconds: 10, extra: bookmarkHotkey(onAdd) });
    return f;
  }

  it("is free: no default binding uses b", () => {
    const keys = Object.values(defaultHotkeys(10)).flatMap((h) => h.keys.map((k) => k.toLowerCase()));
    expect(keys).not.toContain("b");
  });

  it("asks the page to add a bookmark on b and B, and leaves playback alone", () => {
    const onAdd = vi.fn();
    const { state } = withBookmarkKey(onAdd);
    const e = press(document, "b");
    press(document, "B");
    expect(onAdd).toHaveBeenCalledTimes(2);
    expect(e.defaultPrevented).toBe(true);
    expect(state.paused).toBe(true);
  });

  it("does nothing while typing in a field, or with a modifier", () => {
    const onAdd = vi.fn();
    withBookmarkKey(onAdd);
    const input = document.body.appendChild(document.createElement("input"));
    input.focus();
    press(input, "b");
    input.blur();
    press(document, "b", { ctrlKey: true });
    expect(onAdd).not.toHaveBeenCalled();
  });

  it("is absent without the page opting in", () => {
    const f = fakePlayer({ duration: 200 });
    document.body.appendChild(f.el);
    detach = attachHotkeys(f.player, { scope: "page", seekSeconds: 10 });
    const e = press(document, "b");
    expect(e.defaultPrevented).toBe(false);
  });
});
