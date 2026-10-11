import { createOverlayIcon } from "./overlay-icon";
import { stepRate } from "./speeds";
import type { PlayerLike } from "./types";

const clamp = (value: number, min: number, max: number) => Math.min(Math.max(value, min), max);
const vjsIcon = (name: string) => `vjs-icon-${name}`;
const faIcon = (name: string) => `fa-solid fa-${name}`;

/** What a handler may return: the class string of an overlay icon to flash. */
type Icon = string | undefined;

export interface Hotkey {
  /** The `KeyboardEvent.key` values that trigger it (old browsers' names included). */
  keys: string[];
  /** Overrides `keys` for keys that are not a fixed list (the digits). */
  match?: (event: KeyboardEvent) => number | undefined;
  handle(player: PlayerLike, matched: string | number | undefined): Icon | void;
  /** Evaluated after `handle`, for icons that depend on the new state. */
  icon?: (player: PlayerLike) => string;
  /** What the shortcuts popup shows. */
  action: string;
  label: string;
}

/**
 * The key bindings of the old player (web/ts/hotkeys.ts), unchanged, so muscle memory
 * carries over. Rebinding is a parent's choice: pass `extra` to {@link attachHotkeys}
 * to add an action or replace one by name.
 */
export function defaultHotkeys(seekSeconds: number): Record<string, Hotkey> {
  const seekBy = (delta: number) => (player: PlayerLike) => {
    const ranges = player.seekable();
    // A live stream's duration is Infinity; what can be reached is its seekable range.
    const upper = ranges.length ? ranges.end(ranges.length - 1) : player.duration();
    const target = player.currentTime() + delta;
    player.currentTime(clamp(target, 0, Number.isFinite(upper) ? upper : target));
  };
  const volumeBy = (delta: number) => (player: PlayerLike) => {
    const before = player.volume();
    player.volume(clamp(before + delta, 0, 1));
    player.muted(false);
    // Only flash an icon when the volume actually moved.
    if ((delta > 0 && before < 1) || (delta < 0 && before > 0)) {
      return vjsIcon(`volume-${player.volume() === 0 ? "mute" : delta > 0 ? "high" : "mid"}`);
    }
  };
  const rateBy = (faster: boolean) => (player: PlayerLike) => {
    const next = stepRate(player.playbackRates(), player.playbackRate(), faster);
    if (next !== undefined) player.playbackRate(next);
  };
  const seekToFraction = (fraction: number) => (player: PlayerLike) => {
    const ranges = player.seekable();
    if (!ranges.length) return;
    const wasEnded = player.ended();
    const time = timeAtFraction(ranges, clamp(fraction, 0, 1));
    if (time !== undefined) player.currentTime(time);
    if (wasEnded) void player.play();
  };

  return {
    fullscreen: {
      keys: ["f", "F"],
      action: "Open/Exit toggle fullscreen",
      label: "F / f",
      handle(player) {
        // The old state: a toggle does not always complete synchronously.
        const was = player.isFullscreen();
        if (was) void player.exitFullscreen();
        else void player.requestFullscreen();
        return vjsIcon(was ? "fullscreen-exit" : "fullscreen-enter");
      },
    },
    mute: {
      keys: ["m", "M"],
      action: "Mute/Unmute",
      label: "M / m",
      handle: (player) => player.muted(!player.muted()),
      icon: (player) => vjsIcon(player.muted() || player.volume() === 0 ? "volume-mute" : "volume-high"),
    },
    // "Spacebar" is for IE and old Firefox, as are "Right", "Left", "Up" and "Down" below.
    playPause: {
      keys: ["k", "K", " ", "Spacebar"],
      action: "Toggle play/pause",
      label: "K / k / SPACEBAR",
      handle(player) {
        if (player.paused()) void player.play();
        else player.pause();
      },
      icon: (player) => vjsIcon(player.paused() ? "pause" : "play"),
    },
    seekForward: {
      keys: ["l", "L", "ArrowRight", "Right"],
      action: "Seek forward",
      label: "L / l / ArrowRight / Right",
      handle: seekBy(seekSeconds),
      icon: () => `${vjsIcon("replay")} -scale-x-100 rotate-45`,
    },
    seekBack: {
      keys: ["j", "J", "ArrowLeft", "Left"],
      action: "Seek backwards",
      label: "J / j / ArrowLeft / Left",
      handle: seekBy(-seekSeconds),
      icon: () => `${vjsIcon("replay")} -rotate-45`,
    },
    volumeUp: { keys: ["ArrowUp", "Up"], action: "Volume up", label: "ArrowUp / Up", handle: volumeBy(0.05) },
    volumeDown: {
      keys: ["ArrowDown", "Down"],
      action: "Volume down",
      label: "ArrowDown / Down",
      handle: volumeBy(-0.05),
    },
    increasePlaybackRate: {
      keys: [">"],
      action: "Increase playback rate",
      label: ">",
      handle: rateBy(true),
      icon: () => faIcon("forward"),
    },
    decreasePlaybackRate: {
      keys: ["<"],
      action: "Decrease playback rate",
      label: "<",
      handle: rateBy(false),
      icon: () => faIcon("backward"),
    },
    seekPercentage: {
      keys: [],
      match: (event) => (/^[0-9]$/.test(event.key) ? Number(event.key) / 10 : undefined),
      action: "Seek to specific point in the video (7 advances to 70% of duration)",
      label: "0…9",
      handle: (player, fraction) => seekToFraction(fraction as number)(player),
    },
    seekStart: {
      keys: ["Home"],
      action: "Seek to the start",
      label: "Home",
      handle: (player) => seekToFraction(0)(player),
    },
    seekEnd: {
      keys: ["End"],
      action: "Seek to the end",
      label: "End",
      handle: (player) => seekToFraction(1)(player),
    },
  };
}

/** The rows of the shortcuts popup (KeyboardShortcuts.vue), derived from the bindings. */
export const HOTKEY_HELP: { action: string; keys: string }[] = Object.values(defaultHotkeys(0)).map((h) => ({
  action: h.action,
  keys: h.label,
}));

/**
 * The time `fraction` of the way through the playable ranges. In practice a recording
 * has one range [0, duration], but a clip or a partial live window may have more.
 */
export function timeAtFraction(ranges: TimeRanges, fraction: number): number | undefined {
  const lengths = Array.from({ length: ranges.length }, (_, i) => ranges.end(i) - ranges.start(i));
  const target = lengths.reduce((a, b) => a + b, 0) * fraction;
  let passed = 0;
  for (let i = 0; i < ranges.length; i++) {
    if (passed + lengths[i] >= target) return ranges.start(i) + (target - passed);
    passed += lengths[i];
  }
  return undefined;
}

/** The elements whose own key handling a page-wide hotkey must not hijack. */
export function isTypingTarget(el: Element | null): boolean {
  if (!el) return false;
  if ((el as HTMLElement).isContentEditable) return true;
  const tag = el.tagName.toLowerCase();
  if (tag === "textarea" || tag === "select") return true;
  if (tag !== "input") return false;
  const nonText = ["button", "checkbox", "hidden", "radio", "reset", "submit"];
  return !nonText.includes((el as HTMLInputElement).type);
}

export interface HotkeyOptions {
  /**
   * "player": only while focus is inside the player. "page": from anywhere on the page,
   * which is what the old player did -- but with two players only one of them may use it.
   */
  scope: "player" | "page";
  /** The skip distance of the seek keys; also the one on the skip buttons. */
  seekSeconds: number;
  /** Added to the defaults; an entry with a default's name replaces it. */
  extra?: Record<string, Hotkey>;
}

/** Returns the function that detaches the handler and removes the overlay icon. */
export function attachHotkeys(player: PlayerLike, options: HotkeyOptions): () => void {
  const hotkeys = { ...defaultHotkeys(options.seekSeconds), ...options.extra };
  const overlay = createOverlayIcon(player.el());
  const target: EventTarget = options.scope === "page" ? document : player.el();

  const onKeyDown = (event: Event) => {
    const e = event as KeyboardEvent;
    // Leave browser shortcuts alone (ctrl+L, cmd+F, ...).
    if (e.altKey || e.ctrlKey || e.metaKey) return;
    if (isTypingTarget(document.activeElement)) return;
    // Arrow keys move within an open menu (the speed menu) and must stay there.
    if (e.target instanceof Element && e.target.closest(".vjs-menu")) return;
    const active = document.activeElement;
    // Space on a focused control would also "click" it after we toggled playback; blur
    // it first. See https://github.com/videojs/video.js/issues/2669
    if ((e.key === " " || e.key === "Spacebar") && active instanceof HTMLElement && active.closest(".vjs-control-bar")) {
      active.blur();
    }
    for (const hotkey of Object.values(hotkeys)) {
      const matched = hotkey.match ? hotkey.match(e) : hotkey.keys.includes(e.key) ? e.key : undefined;
      if (matched === undefined) continue;
      e.preventDefault();
      e.stopPropagation();
      const fromHandler = hotkey.handle(player, matched);
      const icon = hotkey.icon ? hotkey.icon(player) : fromHandler;
      if (icon) overlay.show(icon);
      return;
    }
  };

  // A page-wide handler captures, so it runs before anything inside the page stops the
  // event; a player-scoped one bubbles, so the sliders and menus inside the player handle
  // their own keys first.
  const capture = options.scope === "page";
  target.addEventListener("keydown", onKeyDown, capture);
  return () => {
    target.removeEventListener("keydown", onKeyDown, capture);
    overlay.dispose();
  };
}
