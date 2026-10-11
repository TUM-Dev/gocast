import { flushPromises, mount } from "@vue/test-utils";
import { beforeEach, describe, expect, it, vi } from "vitest";

import { fakePlayer } from "@/test/fake-player";

import VideoPlayer from "./VideoPlayer.vue";

type Handler = () => void;

const state = vi.hoisted(() => ({
  /** Fires every handler registered for the event, as video.js would. */
  handlers: {} as Record<string, Handler>,
  src: vi.fn(),
  dispose: vi.fn(),
  options: undefined as unknown,
  fake: undefined as unknown as ReturnType<typeof import("@/test/fake-player").fakePlayer>,
  safari: false,
  ios: false,
  spriteThumbnails: vi.fn(),
  addRemoteTextTrack: vi.fn(() => ({ track: {} })),
  removeRemoteTextTrack: vi.fn(),
  levels: [] as { id: string; height: number; width: number; bitrate: number; enabled: boolean }[],
  length: 0,
  duration: NaN,
}));

vi.mock("video.js/dist/video-js.css", () => ({}));
vi.mock("./player.css", () => ({}));
vi.mock("videojs-sprite-thumbnails", () => ({}));
vi.mock("@silvermine/videojs-airplay", () => ({ default: vi.fn() }));
vi.mock("@silvermine/videojs-airplay/dist/silvermine-videojs-airplay.css", () => ({}));
vi.mock("video.js", () => {
  const videojs = (_el: HTMLElement, options: unknown) => {
    state.options = options;
    const { player } = state.fake;
    const all: Record<string, Handler[]> = {};
    const on = player.on.bind(player);
    // Wrap `on` so the test can fire the handlers of one event by name.
    (player as unknown as { on: unknown }).on = (event: string | string[], fn: Handler) => {
      for (const name of Array.isArray(event) ? event : [event]) {
        (all[name] ||= []).push(fn);
        state.handlers[name] = () => all[name].forEach((f) => f());
      }
      on(event, fn);
    };
    return Object.assign(player, {
      src: state.src,
      dispose: state.dispose,
      spriteThumbnails: state.spriteThumbnails,
      addRemoteTextTrack: state.addRemoteTextTrack,
      removeRemoteTextTrack: state.removeRemoteTextTrack,
      qualityLevels: () => Object.assign(state.levels, { on: vi.fn(), off: vi.fn() }),
      getChild: () => undefined,
      controlBar: { getChild: () => undefined, children: () => [] },
    });
  };
  return {
    default: Object.assign(videojs, {
      browser: {
        get IS_SAFARI() {
          return state.safari;
        },
        get IS_IOS() {
          return state.ios;
        },
      },
    }),
  };
});

beforeEach(() => {
  state.handlers = {};
  state.fake = fakePlayer({ duration: NaN });
  state.safari = false;
  state.ios = false;
  state.levels = [];
  for (const fn of [state.src, state.dispose, state.spriteThumbnails, state.addRemoteTextTrack, state.removeRemoteTextTrack]) {
    fn.mockClear();
  }
  localStorage.clear();
});

async function mountPlayer(props: Record<string, unknown> = {}) {
  const wrapper = mount(VideoPlayer, { props: { src: "/a.m3u8", ...props }, attachTo: document.body });
  await flushPromises();
  return wrapper;
}

type Exposed = {
  currentTime: number;
  duration: number;
  seek(s: number): void;
  play(): Promise<void>;
  pause(): void;
  setRate(r: number): void;
  getRate(): number;
  getCurrentTime(): number;
  on(event: string, fn: () => void): () => void;
  toRecordingTime(t: number): number;
};
const api = (wrapper: { vm: unknown }) => wrapper.vm as unknown as Exposed;

describe("VideoPlayer", () => {
  it("loads the source as HLS", async () => {
    await mountPlayer();
    expect(state.src).toHaveBeenCalledWith({ src: "/a.m3u8", type: "application/x-mpegURL" });
  });

  it("passes poster and autoplay on", async () => {
    await mountPlayer({ poster: "/p.jpg", autoplay: true });
    expect(state.options).toMatchObject({ poster: "/p.jpg", autoplay: true, controls: true });
  });

  it("is a plain player unless a feature is asked for", async () => {
    await mountPlayer();
    expect(state.options).toEqual({
      controls: true,
      fluid: true,
      preload: "metadata",
      autoplay: false,
      poster: undefined,
    });
    // ... and listens to nothing it was not asked to: no storage, no hotkeys.
    state.fake.player.volume(0.3);
    state.fake.player.playbackRate(1.5);
    expect(localStorage.length).toBe(0);
    state.fake.el.dispatchEvent(new KeyboardEvent("keydown", { key: "k", bubbles: true }));
    expect(state.fake.state.paused).toBe(true);
  });

  it("reloads when the source changes", async () => {
    const wrapper = await mountPlayer();
    await wrapper.setProps({ src: "/b.m3u8" });
    expect(state.src).toHaveBeenLastCalledWith({ src: "/b.m3u8", type: "application/x-mpegURL" });
  });

  it("exposes the position and emits timeupdate", async () => {
    const wrapper = await mountPlayer();
    state.fake.state.time = 12.5;
    state.handlers.timeupdate();
    expect(wrapper.emitted("timeupdate")).toEqual([[12.5]]);
    expect(api(wrapper).currentTime).toBe(12.5);
  });

  it("reports the length only once it is known", async () => {
    const wrapper = await mountPlayer();
    state.handlers.loadedmetadata();
    expect(wrapper.emitted("loadedmetadata")).toBeUndefined();
    state.fake.state.duration = 90.4;
    state.handlers.loadedmetadata();
    expect(wrapper.emitted("loadedmetadata")).toEqual([[90.4]]);
    expect(api(wrapper).duration).toBe(90.4);
  });

  it("seeks in seconds", async () => {
    const wrapper = await mountPlayer();
    api(wrapper).seek(30);
    expect(state.fake.state.time).toBe(30);
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

describe("VideoPlayer options", () => {
  it("offers the given speeds, capped at 2x on Safari", async () => {
    await mountPlayer({ playbackRates: [2, 0.5, 1, 3] });
    expect(state.options).toMatchObject({ playbackRates: [0.5, 1, 2, 3] });
    state.safari = true;
    await mountPlayer({ playbackRates: [2, 0.5, 1, 3] });
    expect(state.options).toMatchObject({ playbackRates: [0.5, 1, 2] });
  });

  it("adds skip buttons with the given distance", async () => {
    await mountPlayer({ skipSeconds: 15 });
    expect(state.options).toMatchObject({ controlBar: { skipButtons: { forward: 15, backward: 15 } } });
  });

  it("turns on the live UI", async () => {
    await mountPlayer({ live: true });
    expect(state.options).toMatchObject({ liveui: true });
  });

  it("uses iOS' native text tracks for subtitles", async () => {
    state.ios = true;
    await mountPlayer({ subtitles: [{ src: "/de.vtt", lang: "de" }] });
    expect(state.options).toMatchObject({ html5: { nativeTextTracks: true } });
  });
});

describe("VideoPlayer settings", () => {
  it("restores and keeps the viewer's volume, mute and speed", async () => {
    localStorage.setItem("volume", "0.4");
    localStorage.setItem("muted", "true");
    localStorage.setItem("rate", "1.5");
    await mountPlayer({ persist: true });
    expect(state.fake.state).toMatchObject({ volume: 0.4, muted: true, rate: 1.5 });
    state.fake.player.volume(0.9);
    state.fake.player.playbackRate(2);
    expect(localStorage.getItem("volume")).toBe("0.9");
    expect(localStorage.getItem("rate")).toBe("2");
  });

  it("keeps a live stream's speed apart", async () => {
    localStorage.setItem("rate", "2");
    localStorage.setItem("live_rate", "1.25");
    await mountPlayer({ persist: true, live: true });
    expect(state.fake.state.rate).toBe(1.25);
  });

  it("starts at the page's rate over the stored one", async () => {
    localStorage.setItem("rate", "1.5");
    await mountPlayer({ persist: true, rate: 2 });
    expect(state.fake.state.rate).toBe(2);
  });

  it("starts at the page's start time", async () => {
    await mountPlayer({ startTime: 90 });
    state.fake.state.duration = 600;
    state.handlers.loadedmetadata();
    expect(state.fake.state.time).toBe(90);
  });

  it("does not seek for a start time of zero", async () => {
    await mountPlayer({ startTime: 0 });
    state.fake.state.time = 7;
    state.handlers.loadedmetadata();
    expect(state.fake.state.time).toBe(7);
  });

  it("holds to 2x without storing it", async () => {
    vi.useFakeTimers();
    try {
      await mountPlayer({ persist: true, holdToFast: true });
      state.fake.el.dispatchEvent(new MouseEvent("mousedown", { bubbles: true }));
      vi.advanceTimersByTime(250);
      expect(state.fake.state.rate).toBe(2);
      expect(localStorage.getItem("rate")).not.toBe("2");
      document.dispatchEvent(new MouseEvent("mouseup", { bubbles: true }));
      expect(state.fake.state.rate).toBe(1);
    } finally {
      vi.useRealTimers();
    }
  });
});

describe("VideoPlayer hotkeys", () => {
  it("handles keys in the player when asked, and not after unmount", async () => {
    const wrapper = await mountPlayer({ hotkeys: "player", skipSeconds: 10 });
    state.fake.el.dispatchEvent(new KeyboardEvent("keydown", { key: "k", bubbles: true }));
    expect(state.fake.state.paused).toBe(false);
    wrapper.unmount();
    state.fake.el.dispatchEvent(new KeyboardEvent("keydown", { key: "k", bubbles: true }));
    expect(state.fake.state.paused).toBe(false);
  });
});

describe("VideoPlayer stats panel", () => {
  const panel = () => state.fake.el.querySelector<HTMLElement>(".vjs-stats-panel");

  it("adds nothing unless asked for", async () => {
    await mountPlayer({ hotkeys: "player" });
    expect(panel()).toBeNull();
    expect(state.fake.el.querySelector(".vjs-context-menu")).toBeNull();
    state.fake.el.dispatchEvent(new KeyboardEvent("keydown", { key: "i", bubbles: true }));
  });

  it("toggles from the I key and from the right-click menu", async () => {
    await mountPlayer({ statsPanel: true, hotkeys: "player" });
    expect(panel()!.hidden).toBe(true);
    state.fake.el.dispatchEvent(new KeyboardEvent("keydown", { key: "i", bubbles: true }));
    expect(panel()!.hidden).toBe(false);

    state.fake.el.dispatchEvent(new MouseEvent("contextmenu", { bubbles: true, cancelable: true }));
    const entry = state.fake.el.querySelector<HTMLButtonElement>(".vjs-context-menu button")!;
    expect(entry.textContent).toBe("Hide stats for nerds");
    entry.click();
    expect(panel()!.hidden).toBe(true);
  });

  it("works without hotkeys, through the exposed toggle", async () => {
    const wrapper = await mountPlayer({ statsPanel: true });
    (wrapper.vm as unknown as { toggleStats(): void }).toggleStats();
    expect(panel()!.hidden).toBe(false);
  });

  it("is removed with the player", async () => {
    const wrapper = await mountPlayer({ statsPanel: true });
    wrapper.unmount();
    expect(panel()).toBeNull();
    expect(state.fake.el.querySelector(".vjs-context-menu")).toBeNull();
  });
});

describe("VideoPlayer plugins and tracks", () => {
  it("initialises the seekbar preview from the sprite", async () => {
    await mountPlayer({ sprites: { url: "/thumbs/1", interval: 10 } });
    expect(state.spriteThumbnails).toHaveBeenCalledWith({
      url: "/thumbs/1",
      interval: 10,
      width: 160,
      height: 90,
      columns: 17,
    });
  });

  it("has no preview without a sprite", async () => {
    await mountPlayer();
    expect(state.spriteThumbnails).not.toHaveBeenCalled();
  });

  it("adds subtitle tracks and replaces them when they change", async () => {
    const wrapper = await mountPlayer({ subtitles: [{ src: "/de.vtt", lang: "de", label: "Deutsch" }] });
    expect(state.addRemoteTextTrack).toHaveBeenCalledOnce();
    await wrapper.setProps({ subtitles: [{ src: "/en.vtt", lang: "en" }] });
    expect(state.removeRemoteTextTrack).toHaveBeenCalledOnce();
    expect(state.addRemoteTextTrack).toHaveBeenCalledTimes(2);
  });

  it("pins the highest rendition when asked", async () => {
    state.levels = [
      { id: "a", height: 360, width: 640, bitrate: 1, enabled: true },
      { id: "b", height: 1080, width: 1920, bitrate: 2, enabled: true },
    ];
    const wrapper = await mountPlayer({ quality: "highest" });
    expect(state.levels.map((l) => l.enabled)).toEqual([false, true]);
    await wrapper.setProps({ quality: "auto" });
    expect(state.levels.map((l) => l.enabled)).toEqual([true, true]);
  });
});

describe("VideoPlayer clip", () => {
  it("plays only the clip: starts at its start and reports clip time", async () => {
    state.fake = fakePlayer({ duration: 600 });
    const wrapper = await mountPlayer({ clip: { start: 100, end: 160 } });
    expect(state.fake.state.time).toBe(100);
    expect(state.fake.player.duration()).toBe(60);
    state.fake.state.time = 130;
    state.handlers.timeupdate();
    expect(wrapper.emitted("timeupdate")?.at(-1)).toEqual([30]);
    expect(api(wrapper).toRecordingTime(30)).toBe(130);
  });

  it("stops at the end of the clip", async () => {
    state.fake = fakePlayer({ duration: 600 });
    await mountPlayer({ clip: { start: 100, end: 160 } });
    await state.fake.player.play();
    state.fake.state.time = 160.1;
    state.handlers.timeupdate();
    expect(state.fake.state.paused).toBe(true);
  });

  it("moves to a new clip's start when the clip changes", async () => {
    state.fake = fakePlayer({ duration: 600 });
    const wrapper = await mountPlayer({ clip: { start: 100, end: 160 } });
    await wrapper.setProps({ clip: { start: 300, end: 360 } });
    expect(state.fake.state.time).toBe(300);
    await wrapper.setProps({ clip: undefined });
    expect(state.fake.player.duration()).toBe(600);
  });

  it("has no seekbar preview for a clip that does not start at 0", async () => {
    state.fake = fakePlayer({ duration: 600 });
    await mountPlayer({ clip: { start: 100 }, sprites: { url: "/s", interval: 5 } });
    expect(state.spriteThumbnails).not.toHaveBeenCalled();
  });
});

describe("VideoPlayer imperative API", () => {
  it("plays, pauses, seeks and sets the speed", async () => {
    const wrapper = await mountPlayer();
    await api(wrapper).play();
    expect(state.fake.state.paused).toBe(false);
    api(wrapper).pause();
    expect(state.fake.state.paused).toBe(true);
    api(wrapper).setRate(1.5);
    expect(api(wrapper).getRate()).toBe(1.5);
    api(wrapper).seek(42);
    expect(api(wrapper).getCurrentTime()).toBe(42);
  });

  it("delivers player events to subscribers", async () => {
    const wrapper = await mountPlayer();
    const onPause = vi.fn();
    const off = api(wrapper).on("pause", onPause);
    await api(wrapper).play();
    api(wrapper).pause();
    expect(onPause).toHaveBeenCalledOnce();
    off();
    await api(wrapper).play();
    api(wrapper).pause();
    expect(onPause).toHaveBeenCalledOnce();
  });
});
