import { mount } from "@vue/test-utils";
import { afterEach, describe, expect, it, vi } from "vitest";
import { defineComponent, h, ref } from "vue";

import { useClickOutside } from "./useClickOutside";

function mountWithTarget(handler: () => void) {
  const target = ref<HTMLElement | null>(null);
  const wrapper = mount(
    defineComponent({
      setup() {
        useClickOutside(target, handler);
        return () =>
          h("div", [
            h("div", { ref: target }, [h("span", { class: "inside" }, "inside")]),
            h("span", { class: "outside" }, "outside"),
          ]);
      },
    }),
    { attachTo: document.body },
  );
  return { wrapper, target };
}

const press = (el: Element) => el.dispatchEvent(new Event("pointerdown", { bubbles: true }));

afterEach(() => {
  document.body.innerHTML = "";
  vi.restoreAllMocks();
});

describe("useClickOutside", () => {
  it("calls the handler for a press outside the target", () => {
    const handler = vi.fn();
    const { wrapper } = mountWithTarget(handler);

    press(wrapper.find(".outside").element);

    expect(handler).toHaveBeenCalledOnce();
    wrapper.unmount();
  });

  it("ignores a press inside the target", () => {
    const handler = vi.fn();
    const { wrapper } = mountWithTarget(handler);

    press(wrapper.find(".inside").element);

    expect(handler).not.toHaveBeenCalled();
    wrapper.unmount();
  });

  it("listens for pointerdown rather than click", () => {
    const handler = vi.fn();
    const { wrapper } = mountWithTarget(handler);

    wrapper.find(".outside").element.dispatchEvent(new Event("click", { bubbles: true }));

    expect(handler).not.toHaveBeenCalled();
    wrapper.unmount();
  });

  it("does nothing while there is no target element", () => {
    const handler = vi.fn();
    const target = ref<HTMLElement | null>(null);
    const wrapper = mount(
      defineComponent({
        setup() {
          useClickOutside(target, handler);
          return () => h("div", "no menu yet");
        },
      }),
      { attachTo: document.body },
    );

    press(wrapper.element);

    expect(handler).not.toHaveBeenCalled();
    wrapper.unmount();
  });

  it("adds the listener on mount and removes it again on unmount", () => {
    const add = vi.spyOn(document, "addEventListener");
    const remove = vi.spyOn(document, "removeEventListener");
    const handler = vi.fn();
    const { wrapper } = mountWithTarget(handler);

    const added = add.mock.calls.find(([type]) => type === "pointerdown");
    expect(added).toBeDefined();

    wrapper.unmount();

    expect(remove).toHaveBeenCalledWith("pointerdown", added?.[1]);
    press(document.body);
    expect(handler).not.toHaveBeenCalled();
  });
});
