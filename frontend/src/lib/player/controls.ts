import type { PlayerLike } from "./types";

/** The slice of a video.js Component the control bar tweaks need. */
export interface ComponentLike {
  controlText?(text: string): void;
  getChild(name: string): ComponentLike | undefined;
  children(): ComponentLike[];
  removeChild(child: ComponentLike): void;
  addChild(child: ComponentLike, options: object, index: number): void;
}

/**
 * video.js puts the play button first, with the skip buttons around the volume panel.
 * The old player moves it between the two skip buttons, back | play | forward.
 */
export function centrePlayToggle(controlBar: ComponentLike): void {
  const playToggle = controlBar.getChild("playToggle");
  const skipBackward = controlBar.getChild("skipBackward");
  if (!playToggle || !skipBackward) return;
  const index = controlBar.children().indexOf(skipBackward);
  if (index === -1) return;
  controlBar.removeChild(playToggle);
  controlBar.addChild(playToggle, {}, index);
}

/**
 * Tooltips and screen-reader names that name the hotkey ("Pause (k)"), kept current as
 * the state changes. Only worth doing with hotkeys on. Returns the function that stops
 * updating them.
 */
export function labelControlsWithHotkeys(player: PlayerLike, controlBar: ComponentLike, skipSeconds: number): () => void {
  const text = (name: string | string[], value: string) => {
    const path = Array.isArray(name) ? name : [name];
    const component = path.reduce<ComponentLike | undefined>((node, n) => node?.getChild(n), controlBar);
    component?.controlText?.(value);
  };
  if (skipSeconds > 0) {
    text("skipBackward", `Seek back ${skipSeconds} seconds (j)`);
    text("skipForward", `Seek forward ${skipSeconds} seconds (l)`);
  }
  const update = () => {
    text("playToggle", player.paused() ? "Play (k)" : "Pause (k)");
    text(["volumePanel", "muteToggle"], player.muted() ? "Unmute (m)" : "Mute (m)");
    text("fullscreenToggle", player.isFullscreen() ? "Exit Fullscreen (f)" : "Fullscreen (f)");
  };
  const events = ["play", "pause", "volumechange", "fullscreenchange"];
  update();
  player.on(events, update);
  return () => player.off(events, update);
}
