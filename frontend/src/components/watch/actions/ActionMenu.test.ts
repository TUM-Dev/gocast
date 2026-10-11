import { mount } from "@vue/test-utils";
import { afterEach, describe, expect, it, vi } from "vitest";

import ActionMenu from "./ActionMenu.vue";

afterEach(() => vi.restoreAllMocks());

/** jsdom has no layout: answer getBoundingClientRect for the menu as if it were laid out here. */
function menuAt(rect: { left: number; right: number; top: number }) {
  vi.spyOn(HTMLElement.prototype, "getBoundingClientRect").mockImplementation(function (this: HTMLElement) {
    if (this.getAttribute("role") !== "menu") return new DOMRect();
    return { ...rect, bottom: rect.top + 100, width: rect.right - rect.left, height: 100, x: rect.left, y: rect.top, toJSON() {} };
  });
  Object.defineProperty(document.documentElement, "clientWidth", { configurable: true, value: 390 });
  const w = mount(ActionMenu, { props: { label: "Menu", icon: "fa-x" }, slots: { default: "<a role='menuitem'>x</a>" }, attachTo: document.body });
  return w;
}

async function openMenu(w: ReturnType<typeof menuAt>) {
  await w.find("button").trigger("click");
  await new Promise((r) => setTimeout(r));
  return w.find("[role=menu]");
}

describe("ActionMenu", () => {
  it("nudges a menu that runs off the left edge back inside", async () => {
    const w = menuAt({ left: -124, right: 132, top: 50 });
    const menu = await openMenu(w);
    expect(menu.attributes("style")).toContain("translateX(132px)");
    w.unmount();
  });

  it("nudges a menu that runs off the right edge back inside", async () => {
    const w = menuAt({ left: 200, right: 456, top: 50 });
    const menu = await openMenu(w);
    expect(menu.attributes("style")).toContain("translateX(-74px)");
    w.unmount();
  });

  it("leaves a menu that fits alone", async () => {
    const w = menuAt({ left: 20, right: 276, top: 50 });
    const menu = await openMenu(w);
    expect(menu.attributes("style") ?? "").not.toContain("translate");
    expect(menu.classes()).toContain("bottom-full");
    w.unmount();
  });

  it("opens below when there is no room above", async () => {
    const w = menuAt({ left: 20, right: 276, top: -30 });
    const menu = await openMenu(w);
    expect(menu.classes()).toContain("top-full");
    expect(menu.classes()).not.toContain("bottom-full");
    w.unmount();
  });

  it("shows a visible caption and keeps the menu's own name when asked", () => {
    const w = mount(ActionMenu, { props: { label: "Menu", icon: "fa-x", text: "Current", buttonLabel: "Menu, current: Current" } });
    const btn = w.find("button");
    expect(btn.text()).toBe("Current");
    expect(btn.attributes("aria-label")).toBe("Menu, current: Current");
    w.unmount();
  });

  it("stays an icon-only button without a caption", () => {
    const w = mount(ActionMenu, { props: { label: "Menu", icon: "fa-x" } });
    expect(w.find("button").text()).toBe("");
    expect(w.find("button").attributes("aria-label")).toBe("Menu");
    w.unmount();
  });
});
