import type { Clip } from "./clip";

/** A sprite sheet of seekbar preview frames, one every `interval` seconds. */
export interface SpriteSource {
  url: string;
  /** Seconds between two frames. */
  interval: number;
  /** One frame's size in pixels; the server renders 160x90 frames, 17 per row. */
  width?: number;
  height?: number;
  columns?: number;
}

export interface SpriteThumbnailsOptions {
  url: string;
  interval: number;
  width: number;
  height: number;
  columns: number;
}

/** The phone breakpoint of the old player: previews need a hover, which a phone lacks. */
export const SPRITES_MOBILE_QUERY = "only screen and (max-width: 480px)";

/**
 * The options of videojs-sprite-thumbnails for a sprite, or undefined where the preview
 * should not show: no sprite, a phone, or a clip that starts after 0 (the plugin maps
 * the pointer to a time via the player's duration and cannot be given an offset, so its
 * frames would belong to the wrong minutes).
 */
export function spriteOptions(
  sprite: SpriteSource | undefined,
  context: { mobile: boolean; clip?: Clip },
): SpriteThumbnailsOptions | undefined {
  if (!sprite || !sprite.url || !(sprite.interval > 0) || context.mobile) return undefined;
  if (context.clip && context.clip.start > 0) return undefined;
  return {
    url: sprite.url,
    interval: sprite.interval,
    width: sprite.width ?? 160,
    height: sprite.height ?? 90,
    columns: sprite.columns ?? 17,
  };
}

export function isMobileViewport(): boolean {
  return typeof window !== "undefined" && !!window.matchMedia && window.matchMedia(SPRITES_MOBILE_QUERY).matches;
}

interface TooltipHost {
  getChild(name: string): TooltipHost | undefined;
  el(): Element;
}

/**
 * The preview floats above the time tooltip, which needs room for it: the `thumb` class
 * (styled in player.css) gives it the padding. Quietly does nothing if this video.js
 * has no such component.
 */
export function markTimeTooltip(player: TooltipHost): void {
  const tooltip = ["controlBar", "progressControl", "seekBar", "mouseTimeDisplay", "timeTooltip"].reduce<
    TooltipHost | undefined
  >((node, name) => node?.getChild(name), player);
  tooltip?.el().classList.add("vjs-sprite-thumb");
}
