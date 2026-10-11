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
  ): { track: unknown } | undefined;
  removeRemoteTextTrack(track: unknown): void;
}

/**
 * Makes the player's subtitle tracks exactly `tracks`, replacing what an earlier call
 * added, and returns the function that removes them. iOS plays text in its native
 * renderer, which VideoPlayer.vue enables for it (`nativeTextTracks`); the tracks are
 * added the same way.
 */
export function setSubtitleTracks(player: TextTrackHost, tracks: readonly SubtitleTrack[]): () => void {
  const added: unknown[] = [];
  for (const t of tracks) {
    const element = player.addRemoteTextTrack(
      { kind: "subtitles", src: t.src, srclang: t.lang, label: t.label ?? t.lang, default: t.default },
      // true: video.js drops a track that is not manual-cleanup whenever the source
      // changes, which silently emptied the captions menu when the player loaded its
      // source after the tracks were added. We remove them ourselves.
      true,
    );
    // video.js returns nothing while it has no tech yet, i.e. before the first source.
    if (element) added.push(element.track);
  }
  return () => {
    for (const track of added) player.removeRemoteTextTrack(track);
  };
}
