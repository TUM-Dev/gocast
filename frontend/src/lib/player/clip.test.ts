import { describe, expect, it } from "vitest";

import { fakePlayer } from "@/test/fake-player";

import { installClip } from "./clip";

function clipped(clip: { start: number; end?: number }, duration = 600) {
  const f = fakePlayer({ duration });
  const handle = installClip(f.player, clip);
  return { ...f, handle };
}

describe("installClip", () => {
  it("presents the clip as the whole media", () => {
    const { player, state } = clipped({ start: 100, end: 160 });
    expect(player.duration()).toBe(60);
    expect(player.currentTime()).toBe(0);
    state.time = 130;
    expect(player.currentTime()).toBe(30);
  });

  it("starts at the clip's start", () => {
    const { state } = clipped({ start: 100, end: 160 });
    expect(state.time).toBe(100);
  });

  it("seeks on the clip's clock and keeps within it", () => {
    const { player, state } = clipped({ start: 100, end: 160 });
    player.currentTime(20);
    expect(state.time).toBe(120);
    player.currentTime(500);
    expect(state.time).toBe(160);
    player.currentTime(-5);
    expect(state.time).toBe(100);
  });

  it("an open-ended clip runs to the end of the recording", () => {
    const { player } = clipped({ start: 100 });
    expect(player.duration()).toBe(500);
  });

  it("an end past the recording is the recording's end", () => {
    const { player } = clipped({ start: 100, end: 9999 });
    expect(player.duration()).toBe(500);
  });

  it("keeps video.js' own duration setter working, so durationchange still fires", () => {
    const { player, state } = clipped({ start: 10, end: 20 });
    player.duration(300);
    expect(state.duration).toBe(300);
    expect(player.duration()).toBe(10);
  });

  it("pauses at the end, puts the playhead on it and says it ended", () => {
    const { player, state } = clipped({ start: 100, end: 160 });
    let ended = 0;
    player.on("ended", () => ended++);
    state.paused = false;
    state.time = 159.99;
    player.trigger("timeupdate");
    expect(state.paused).toBe(true);
    expect(state.time).toBe(160);
    expect(ended).toBe(1);
    expect(player.ended()).toBe(true);
    // further updates do not re-fire it
    player.trigger("timeupdate");
    expect(ended).toBe(1);
  });

  it("replays from the start when played again at the end", () => {
    const { player, state } = clipped({ start: 100, end: 160 });
    state.time = 160;
    player.trigger("timeupdate");
    player.play();
    expect(state.time).toBe(100);
    expect(player.ended()).toBe(false);
  });

  it("seeking away from the end clears ended", () => {
    const { player, state } = clipped({ start: 100, end: 160 });
    state.time = 160;
    player.trigger("timeupdate");
    player.currentTime(10);
    expect(player.ended()).toBe(false);
  });

  it("pulls a playhead that landed before the clip to its start", () => {
    const { player, state } = clipped({ start: 100, end: 160 });
    state.time = 0;
    player.trigger("timeupdate");
    expect(state.time).toBe(100);
  });

  it("shifts buffered and seekable ranges into the clip", () => {
    const { player, state } = clipped({ start: 100, end: 160 }, 600);
    // the fake buffers [0, duration / 2] = [0, 300]
    expect([player.buffered().start(0), player.buffered().end(0)]).toEqual([0, 60]);
    expect(player.seekable().length).toBe(1);
    state.duration = 600;
    expect(player.seekable().end(0)).toBe(60);
  });

  it("converts between the clocks", () => {
    const { handle } = clipped({ start: 100, end: 160 });
    expect(handle.toRecordingTime(12)).toBe(112);
    expect(handle.toClipTime(112)).toBe(12);
    expect(handle.toClipTime(5)).toBe(0);
  });

  it("is undone by dispose", () => {
    const { player, state, handle } = clipped({ start: 100, end: 160 });
    const before = player.handlerCount();
    handle.dispose();
    expect(player.handlerCount()).toBeLessThan(before);
    state.time = 130;
    expect(player.currentTime()).toBe(130);
    expect(player.duration()).toBe(600);
  });
});
