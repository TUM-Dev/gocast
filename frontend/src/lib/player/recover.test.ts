import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

import { recoverFromErrors, type RecoverablePlayer } from "./recover";

function fake(code = 2) {
  const handlers = new Map<string, Set<() => void>>();
  const state = { code, time: 42, paused: false, error: true };
  const player = {
    on: (e: string, fn: () => void) => void (handlers.get(e) ?? handlers.set(e, new Set()).get(e)!).add(fn),
    off: (e: string, fn: () => void) => void handlers.get(e)?.delete(fn),
    error: vi.fn((value?: null) => {
      if (value === null) {
        state.error = false;
        return;
      }
      return state.error ? { code: state.code } : null;
    }),
    src: vi.fn(),
    currentTime: vi.fn((to?: number) => {
      if (to === undefined) return state.time;
      state.time = to;
    }),
    paused: () => state.paused,
    play: vi.fn(() => Promise.resolve()),
  };
  const fire = (e: string) => [...(handlers.get(e) ?? [])].forEach((fn) => fn());
  const listeners = () => [...handlers.entries()].filter(([e]) => e !== "error").reduce((n, [, s]) => n + s.size, 0);
  return { player: player as unknown as RecoverablePlayer, mock: player, state, fire, listeners };
}

const source = () => ({ src: "https://edge/vod/a.m3u8?jwt=x", type: "application/x-mpegURL" });

describe("recoverFromErrors", () => {
  beforeEach(() => vi.useFakeTimers());
  afterEach(() => vi.useRealTimers());

  it("reloads the source after the delay and resumes where the viewer was", () => {
    const { player, mock, state, fire } = fake();
    const giveUp = vi.fn();
    recoverFromErrors(player, source, giveUp, { delays: [1000] });

    fire("error");
    expect(mock.src).not.toHaveBeenCalled();
    vi.advanceTimersByTime(1000);
    expect(mock.src).toHaveBeenCalledWith(source());
    expect(state.error).toBe(false);

    state.time = 0;
    fire("loadedmetadata");
    expect(state.time).toBe(42);
    expect(mock.play).toHaveBeenCalledOnce();
    expect(giveUp).not.toHaveBeenCalled();
  });

  it("does not start a paused video", () => {
    const { player, mock, state, fire } = fake();
    state.paused = true;
    recoverFromErrors(player, source, vi.fn(), { delays: [10] });
    fire("error");
    vi.advanceTimersByTime(10);
    fire("loadedmetadata");
    expect(mock.play).not.toHaveBeenCalled();
  });

  it("gives up when the attempts are spent", () => {
    const { player, mock, state, fire } = fake();
    const giveUp = vi.fn();
    recoverFromErrors(player, source, giveUp, { delays: [10, 10] });
    for (let i = 0; i < 2; i++) {
      fire("error");
      vi.advanceTimersByTime(10);
      state.error = true;
    }
    expect(giveUp).not.toHaveBeenCalled();
    fire("error");
    expect(giveUp).toHaveBeenCalledOnce();
    expect(mock.src).toHaveBeenCalledTimes(2);
  });

  it("keeps counting when the metadata loads but the segment at the position keeps failing", () => {
    const { player, mock, state, fire } = fake();
    const giveUp = vi.fn();
    recoverFromErrors(player, source, giveUp, { delays: [10, 10] });
    for (let i = 0; i < 2; i++) {
      state.error = true;
      fire("error");
      vi.advanceTimersByTime(10);
      // The reload resets the playhead, as a real player does.
      state.time = 0;
      fire("loadedmetadata");
      // Every attempt still resumes at the original position, not at 0.
      expect(state.time).toBe(42);
    }
    state.error = true;
    fire("error");
    expect(giveUp).toHaveBeenCalledOnce();
    expect(mock.src).toHaveBeenCalledTimes(2);
  });

  it("gets its attempts back once the seek to the saved position completes", () => {
    const { player, state, fire } = fake();
    const giveUp = vi.fn();
    recoverFromErrors(player, source, giveUp, { delays: [10] });
    for (let i = 0; i < 3; i++) {
      state.error = true;
      fire("error");
      vi.advanceTimersByTime(10);
      fire("loadedmetadata");
      fire("seeked");
    }
    expect(giveUp).not.toHaveBeenCalled();
  });

  it("counts a recording that failed at the start as recovered once it can play", () => {
    const { player, state, fire, listeners } = fake();
    state.time = 0;
    const giveUp = vi.fn();
    recoverFromErrors(player, source, giveUp, { delays: [10] });
    for (let i = 0; i < 2; i++) {
      state.error = true;
      fire("error");
      vi.advanceTimersByTime(10);
      fire("loadedmetadata");
      fire("canplay");
    }
    expect(giveUp).not.toHaveBeenCalled();
    expect(listeners()).toBe(0);
  });

  it("retries a decode error, which is how a segment cut short by a restart looks", () => {
    const { player, mock, fire } = fake(3);
    const giveUp = vi.fn();
    recoverFromErrors(player, source, giveUp, { delays: [10] });
    fire("error");
    vi.advanceTimersByTime(10);
    expect(mock.src).toHaveBeenCalledOnce();
    expect(giveUp).not.toHaveBeenCalled();
  });

  it("resumes a live stream at the live edge, not at the saved position", () => {
    const { player, state, fire } = fake();
    recoverFromErrors(player, source, vi.fn(), { delays: [10], live: true });
    fire("error");
    vi.advanceTimersByTime(10);
    state.time = 0;
    fire("loadedmetadata");
    expect(state.time).toBe(0);
    fire("canplay");
  });

  it("leaves a clean slate after giving up", () => {
    const { player, mock, state, fire, listeners } = fake();
    const giveUp = vi.fn();
    recoverFromErrors(player, source, giveUp, { delays: [10] });
    fire("error");
    vi.advanceTimersByTime(10);
    fire("loadedmetadata");
    state.error = true;
    fire("error");
    expect(giveUp).toHaveBeenCalledOnce();
    expect(listeners()).toBe(0);

    // A later error gets a full budget and its own position.
    state.time = 7;
    state.error = true;
    fire("error");
    vi.advanceTimersByTime(10);
    expect(mock.src).toHaveBeenCalledTimes(2);
    state.time = 0;
    fire("loadedmetadata");
    expect(state.time).toBe(7);
  });

  it("does not retry what a reload cannot fix", () => {
    const { player, mock, fire } = fake(5);
    const giveUp = vi.fn();
    recoverFromErrors(player, source, giveUp, { delays: [10] });
    fire("error");
    expect(giveUp).toHaveBeenCalledOnce();
    expect(mock.src).not.toHaveBeenCalled();
  });

  it("drops a pending retry and its listeners on reset", () => {
    const { player, mock, state, fire, listeners } = fake();
    const recovery = recoverFromErrors(player, source, vi.fn(), { delays: [10] });
    fire("error");
    vi.advanceTimersByTime(10);
    expect(listeners()).toBe(1);

    recovery.reset();
    expect(listeners()).toBe(0);
    state.time = 7;
    fire("loadedmetadata");
    expect(state.time).toBe(7);

    state.error = true;
    fire("error");
    recovery.reset();
    vi.advanceTimersByTime(10);
    expect(mock.src).toHaveBeenCalledOnce();
  });

  it("starts a new streak from the new source's position after a reset", () => {
    const { player, state, fire } = fake();
    const recovery = recoverFromErrors(player, source, vi.fn(), { delays: [10] });
    fire("error");
    recovery.reset();
    state.time = 5;
    state.error = true;
    fire("error");
    vi.advanceTimersByTime(10);
    state.time = 0;
    fire("loadedmetadata");
    expect(state.time).toBe(5);
  });

  it("on iOS applies the saved position again on canplaythrough, after the ?t= seek", () => {
    const { player, state, fire } = fake();
    recoverFromErrors(player, source, vi.fn(), { delays: [10], isIOS: true });
    // start-time.ts' handler, registered long before the retry.
    const startTime = () => (state.time = 5);
    player.on("canplaythrough", startTime);

    fire("error");
    vi.advanceTimersByTime(10);
    fire("loadedmetadata");
    fire("canplaythrough");
    expect(state.time).toBe(42);

    state.time = 50;
    fire("canplaythrough");
    expect(state.time).toBe(5);
  });

  it("stops on dispose", () => {
    const { player, mock, fire } = fake();
    const recovery = recoverFromErrors(player, source, vi.fn(), { delays: [10] });
    fire("error");
    recovery.dispose();
    vi.advanceTimersByTime(10);
    expect(mock.src).not.toHaveBeenCalled();
  });
});
