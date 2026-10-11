import { flushPromises, mount } from "@vue/test-utils";
import { beforeEach, describe, expect, it, vi } from "vitest";

import VideoPlayer from "./VideoPlayer.vue";

type Handler = () => void;

const state = vi.hoisted(() => ({
  handlers: {} as Record<string, Handler>,
  time: 0,
  length: NaN,
  src: vi.fn(),
  dispose: vi.fn(),
  options: undefined as unknown,
}));

vi.mock("video.js/dist/video-js.css", () => ({}));
vi.mock("video.js", () => ({
  default: (_el: HTMLElement, options: unknown) => {
    state.options = options;
    return {
      on: (name: string, fn: Handler) => (state.handlers[name] = fn),
      src: state.src,
      dispose: state.dispose,
      currentTime: (to?: number) => {
        if (to === undefined) return state.time;
        state.time = to;
      },
      duration: () => state.length,
    };
  },
}));

beforeEach(() => {
  state.handlers = {};
  state.time = 0;
  state.length = NaN;
  state.src.mockClear();
  state.dispose.mockClear();
});

async function mountPlayer(props: Record<string, unknown> = {}) {
  const wrapper = mount(VideoPlayer, { props: { src: "/a.m3u8", ...props } });
  await flushPromises();
  return wrapper;
}

describe("VideoPlayer", () => {
  it("loads the source as HLS", async () => {
    await mountPlayer();
    expect(state.src).toHaveBeenCalledWith({ src: "/a.m3u8", type: "application/x-mpegURL" });
  });

  it("passes poster and autoplay on", async () => {
    await mountPlayer({ poster: "/p.jpg", autoplay: true });
    expect(state.options).toMatchObject({ poster: "/p.jpg", autoplay: true, controls: true });
  });

  it("reloads when the source changes", async () => {
    const wrapper = await mountPlayer();
    await wrapper.setProps({ src: "/b.m3u8" });
    expect(state.src).toHaveBeenLastCalledWith({ src: "/b.m3u8", type: "application/x-mpegURL" });
  });

  it("exposes the position and emits timeupdate", async () => {
    const wrapper = await mountPlayer();
    state.time = 12.5;
    state.handlers.timeupdate();
    expect(wrapper.emitted("timeupdate")).toEqual([[12.5]]);
    expect((wrapper.vm as unknown as { currentTime: number }).currentTime).toBe(12.5);
  });

  it("reports the length only once it is known", async () => {
    const wrapper = await mountPlayer();
    state.handlers.loadedmetadata();
    expect(wrapper.emitted("loadedmetadata")).toBeUndefined();
    state.length = 90.4;
    state.handlers.loadedmetadata();
    expect(wrapper.emitted("loadedmetadata")).toEqual([[90.4]]);
    expect((wrapper.vm as unknown as { duration: number }).duration).toBe(90.4);
  });

  it("seeks in seconds", async () => {
    const wrapper = await mountPlayer();
    (wrapper.vm as unknown as { seek(s: number): void }).seek(30);
    expect(state.time).toBe(30);
  });

  it("shows a message when the player errors", async () => {
    const wrapper = await mountPlayer();
    state.handlers.error();
    await flushPromises();
    expect(wrapper.find("[role=status]").exists()).toBe(true);
  });

  it("disposes the player on unmount", async () => {
    const wrapper = await mountPlayer();
    wrapper.unmount();
    expect(state.dispose).toHaveBeenCalledOnce();
  });
});
