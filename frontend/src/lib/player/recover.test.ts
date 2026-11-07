import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

import { recoverFromErrors, type RecoverablePlayer } from "./recover";

function fake(code = 2) {
  const handlers = new Map<string, Set<() => void>>();
  const state = { code, time: 42, paused: false, error: true };
  const player = {
    on: (e: string, fn: () => void) => (handlers.get(e) ?? handlers.set(e, new Set()).get(e)!).add(fn),
    off: (e: string, fn: () => void) => void handlers.get(e)?.delete(fn),
    one(e: string, fn: () => void) {
      const wrapped = () => {
        player.off(e, wrapped);
        fn();
      };
      player.on(e, wrapped);
    },
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
  return { player: player as unknown as RecoverablePlayer, mock: player, state, fire };
}

const source = () => ({ src: "https://edge/vod/a.m3u8?jwt=x", type: "application/x-mpegURL" });

describe("recoverFromErrors", () => {
  beforeEach(() => vi.useFakeTimers());
  afterEach(() => vi.useRealTimers());

  it("reloads the source after the delay and resumes where the viewer was", async () => {
    const { player, mock, state, fire } = fake();
    const giveUp = vi.fn();
    recoverFromErrors(player, source, giveUp, [1000]);

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
    recoverFromErrors(player, source, vi.fn(), [10]);
    fire("error");
    vi.advanceTimersByTime(10);
    fire("loadedmetadata");
    expect(mock.play).not.toHaveBeenCalled();
  });

  it("gives up when the attempts are spent", () => {
    const { player, mock, state, fire } = fake();
    const giveUp = vi.fn();
    recoverFromErrors(player, source, giveUp, [10, 10]);
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

  it("gets its attempts back once a reload works", () => {
    const { player, state, fire } = fake();
    const giveUp = vi.fn();
    recoverFromErrors(player, source, giveUp, [10]);
    for (let i = 0; i < 3; i++) {
      state.error = true;
      fire("error");
      vi.advanceTimersByTime(10);
      fire("loadedmetadata");
    }
    expect(giveUp).not.toHaveBeenCalled();
  });

  it("does not retry what a reload cannot fix", () => {
    const { player, mock, fire } = fake(3);
    const giveUp = vi.fn();
    recoverFromErrors(player, source, giveUp, [10]);
    fire("error");
    expect(giveUp).toHaveBeenCalledOnce();
    expect(mock.src).not.toHaveBeenCalled();
  });

  it("stops on dispose", () => {
    const { player, mock, fire } = fake();
    const stop = recoverFromErrors(player, source, vi.fn(), [10]);
    fire("error");
    stop();
    vi.advanceTimersByTime(10);
    expect(mock.src).not.toHaveBeenCalled();
  });
});
