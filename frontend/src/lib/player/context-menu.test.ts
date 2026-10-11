import { describe, expect, it, vi } from "vitest";

import { fakePlayer } from "@/test/fake-player";

import { attachContextMenu } from "./context-menu";

function setup() {
  const { player, el } = fakePlayer();
  document.body.appendChild(el);
  const run = vi.fn();
  const detach = attachContextMenu(player, [{ label: () => "Stats for nerds", run }]);
  const menu = () => el.querySelector<HTMLElement>(".vjs-context-menu")!;
  const rightClick = (init: MouseEventInit = {}) => {
    const e = new MouseEvent("contextmenu", { bubbles: true, cancelable: true, ...init });
    el.dispatchEvent(e);
    return e;
  };
  return { el, run, detach, menu, rightClick };
}

describe("attachContextMenu", () => {
  it("replaces the browser's menu with its own and runs the chosen entry", () => {
    const { run, menu, rightClick, detach } = setup();
    expect(menu().hidden).toBe(true);
    expect(rightClick().defaultPrevented).toBe(true);
    expect(menu().hidden).toBe(false);
    menu().querySelector("button")!.click();
    expect(run).toHaveBeenCalledOnce();
    expect(menu().hidden).toBe(true);
    detach();
  });

  it("leaves shift + right-click to the browser", () => {
    const { menu, rightClick, detach } = setup();
    expect(rightClick({ shiftKey: true }).defaultPrevented).toBe(false);
    expect(menu().hidden).toBe(true);
    detach();
  });

  it("closes on Escape and on a click elsewhere", () => {
    const { menu, rightClick, detach } = setup();
    rightClick();
    document.dispatchEvent(new KeyboardEvent("keydown", { key: "Escape" }));
    expect(menu().hidden).toBe(true);
    rightClick();
    document.body.dispatchEvent(new Event("pointerdown", { bubbles: true }));
    expect(menu().hidden).toBe(true);
    detach();
  });

  it("is removed on detach", () => {
    const { el, rightClick, detach } = setup();
    detach();
    expect(el.querySelector(".vjs-context-menu")).toBeNull();
    expect(rightClick().defaultPrevented).toBe(false);
  });
});
