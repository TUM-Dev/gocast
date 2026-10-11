/** A subtitle file (WebVTT) for the player's captions menu. */
export interface SubtitleTrack {
  src: string;
  /** BCP 47 language code, e.g. "de". */
  lang: string;
  /** The menu entry; defaults to the language code. */
  label?: string;
  default?: boolean;
}

interface TextTrackHost {
  addRemoteTextTrack(
    options: { kind: string; src: string; srclang: string; label: string; default?: boolean },
    manualCleanup: boolean,
  ): { track: unknown };
  removeRemoteTextTrack(track: unknown): void;
}

/**
 * Makes the player's subtitle tracks exactly `tracks`, replacing what an earlier call
 * added, and returns the function that removes them. iOS plays text in its native
 * renderer, which VideoPlayer.vue enables for it (`nativeTextTracks`); the tracks are
 * added the same way.
 */
export function setSubtitleTracks(player: TextTrackHost, tracks: readonly SubtitleTrack[]): () => void {
  const added = tracks.map(
    (t) =>
      player.addRemoteTextTrack(
        { kind: "subtitles", src: t.src, srclang: t.lang, label: t.label ?? t.lang, default: t.default },
        // false: video.js removes them with the player instead of leaving it to us.
        false,
      ).track,
  );
  return () => {
    for (const track of added) player.removeRemoteTextTrack(track);
  };
}
