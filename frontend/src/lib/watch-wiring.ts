import type { PlayerEvent } from "./player/controller";
import type { PanelPlayer } from "./watch-panels";
import type { WatchController } from "./watch-context";

/**
 * The panels' view of the page's player.
 *
 * Two things differ from handing them the controller itself. The controller speaks the
 * clip's clock while bookmarks, sections and transcript cues are in the recording's, so
 * time is converted both ways (a no-op for an uncut stream). And the controller changes
 * under the sidebar -- another layout remounts the player, the split view replaces it --
 * while a panel subscribes once, so subscriptions are kept here and re-attached to each
 * new controller.
 */
export function createPanelPlayer(current: () => WatchController | null): {
  player: PanelPlayer;
  /** Call when `current()` returns another controller. */
  rebind(): void;
} {
  const listeners = new Map<PlayerEvent, Set<() => void>>();
  let detach: Array<() => void> = [];

  function rebind(): void {
    detach.forEach((off) => off());
    detach = [];
    const c = current();
    if (!c) return;
    for (const [event, fns] of listeners) {
      detach.push(c.on(event, () => [...fns].forEach((fn) => fn())));
    }
  }

  const player: PanelPlayer = {
    getCurrentTime() {
      const c = current();
      return c ? c.toRecordingTime(c.getCurrentTime()) : 0;
    },
    seek(seconds) {
      const c = current();
      if (!c) return;
      // The recording's time to the clip's clock: the inverse of toRecordingTime.
      c.seek(Math.max(0, seconds - (c.toRecordingTime(0) || 0)));
    },
    on(event, fn) {
      let fns = listeners.get(event);
      if (!fns) {
        fns = new Set();
        listeners.set(event, fns);
        const c = current();
        if (c) detach.push(c.on(event, () => [...fns!].forEach((f) => f())));
      }
      fns.add(fn);
      return () => fns!.delete(fn);
    },
  };
  return { player, rebind };
}
