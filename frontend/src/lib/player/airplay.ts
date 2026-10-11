interface VideojsLike {
  registerPlugin?: unknown;
}

let registered = false;

/**
 * Registers the AirPlay button plugin with `videojs`, once. The plugin is the old
 * player's (@silvermine/videojs-airplay, same version range as web/package.json); it is
 * a CommonJS module that takes video.js as an argument, so it is imported on demand by
 * the caller and handed in here.
 */
export function registerAirPlay(videojs: VideojsLike, plugin: (videojs: VideojsLike) => void): void {
  if (registered) return;
  registered = true;
  plugin(videojs);
}

interface AirPlayHost {
  airPlay?: (options: { addButtonToControlBar: boolean; buttonPositionIndex: number }) => void;
}

/**
 * Adds the button, second from the right as before. The plugin shows it only where the
 * browser can AirPlay (Safari), so enabling it elsewhere is harmless.
 */
export function enableAirPlay(player: AirPlayHost): void {
  player.airPlay?.({ addButtonToControlBar: true, buttonPositionIndex: -2 });
}
