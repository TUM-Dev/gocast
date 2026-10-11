import type { PlayerLike } from "./types";

export interface HoldSpeedOptions {
  /** How long the pointer must stay down before the speed-up starts. */
  delayMs?: number;
  rate?: number;
}

const CONTROLS = ".vjs-control-bar, .vjs-control, .vjs-menu, .vjs-modal-dialog";

/**
 * Holding the mouse on the video plays at 2x until it is released (the old player's
 * "hold to fast-forward"). Returns the controller: `active()` is true while sped up,
 * which persistence uses to keep that temporary rate out of localStorage, and
 * `dispose()` removes every listener and timer.
 */
export function attachHoldSpeed(player: PlayerLike, options: HoldSpeedOptions = {}) {
  const delayMs = options.delayMs ?? 200;
  const fastRate = options.rate ?? 2;
  const el = player.el();

  let isActive = false;
  let previousRate: number | null = null;
  let holdTimer: ReturnType<typeof setTimeout> | null = null;
  let releaseFlagTimer: ReturnType<typeof setTimeout> | null = null;
  let justReleased = false;

  const apply = () => {
    if (isActive) return;
    isActive = true;
    previousRate = player.playbackRate();
    if (previousRate !== fastRate) player.playbackRate(fastRate);
  };

  const release = () => {
    if (holdTimer !== null) {
      clearTimeout(holdTimer);
      holdTimer = null;
    }
    if (!isActive) return;
    isActive = false;
    justReleased = true;
    if (previousRate !== null) player.playbackRate(previousRate);
    previousRate = null;
    if (releaseFlagTimer !== null) clearTimeout(releaseFlagTimer);
    // The click that ends the hold fires right after mouseup; swallowing it keeps the
    // video from pausing. The flag only has to outlive that one event.
    releaseFlagTimer = setTimeout(() => {
      justReleased = false;
      releaseFlagTimer = null;
    }, 50);
  };

  const onDown = (event: Event) => {
    const e = event as MouseEvent;
    if (e.button !== 0) return;
    // Only a hold on the picture means "fast forward". The listener sits on the whole
    // player, so without this, dragging the volume or seek slider (or holding any
    // button or menu) also switched to 2x until the mouse was released.
    if (e.target instanceof Element && e.target.closest(CONTROLS)) return;
    if (holdTimer !== null) clearTimeout(holdTimer);
    holdTimer = setTimeout(() => {
      holdTimer = null;
      apply();
    }, delayMs);
  };
  const onUp = (event: Event) => {
    if ((event as MouseEvent).button !== 0) return;
    release();
  };
  const onClick = (event: Event) => {
    if (!justReleased) return;
    event.preventDefault();
    event.stopPropagation();
  };

  el.addEventListener("mousedown", onDown);
  el.addEventListener("click", onClick, true);
  // On the document, so releasing outside the player still ends the hold.
  document.addEventListener("mouseup", onUp);

  return {
    active: () => isActive,
    dispose() {
      if (holdTimer !== null) clearTimeout(holdTimer);
      if (releaseFlagTimer !== null) clearTimeout(releaseFlagTimer);
      holdTimer = releaseFlagTimer = null;
      // Do not leave a viewer stuck at 2x if the player goes away mid-hold.
      if (isActive && previousRate !== null) player.playbackRate(previousRate);
      isActive = false;
      el.removeEventListener("mousedown", onDown);
      el.removeEventListener("click", onClick, true);
      document.removeEventListener("mouseup", onUp);
    },
  };
}
