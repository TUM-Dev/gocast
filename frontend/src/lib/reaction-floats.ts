import { ref, type Ref } from "vue";

/** At most this many emoji float at once; the rest of a burst is dropped, not queued. */
export const MAX_FLOATS = 12;
/** How long one emoji floats, which is the length of the CSS animation. */
export const FLOAT_MS = 2800;
/**
 * How long after the viewer's own reaction the server's copy of it is recognised and
 * skipped. The server sends a reaction to everyone, the sender included, but the sender
 * already saw theirs the moment they pressed it.
 */
const ECHO_MS = 4000;

export interface Float {
  id: number;
  emoji: string;
  /** Horizontal position in percent of the picture's width. */
  left: number;
}

// The reaction picker lives in the action bar and the overlay in the player; they share
// no parent that is not the whole page, so the picker announces here instead.
const ownListeners = new Map<number, Set<(emoji: string) => void>>();

/** Tells the stream's overlay that this viewer just reacted. */
export function announceOwnReaction(streamId: number, emoji: string): void {
  ownListeners.get(streamId)?.forEach((fn) => fn(emoji));
}

/** Listens for {@link announceOwnReaction}; returns the unsubscribe function. */
export function onOwnReaction(streamId: number, fn: (emoji: string) => void): () => void {
  let set = ownListeners.get(streamId);
  if (!set) ownListeners.set(streamId, (set = new Set()));
  set.add(fn);
  return () => set.delete(fn);
}

export interface ReactionFloats {
  floats: Ref<Float[]>;
  /** A reaction that arrived from the server. */
  receive(emoji: string): void;
  /** The viewer's own reaction, shown at once. */
  own(emoji: string): void;
  dispose(): void;
}

/**
 * The emoji currently floating over a stream. Pure state and timers, so the component
 * only renders it. `random` is injectable for tests.
 */
export function createReactionFloats(random: () => number = Math.random): ReactionFloats {
  const floats = ref<Float[]>([]);
  const timers = new Set<ReturnType<typeof setTimeout>>();
  // emoji -> when the server's copy of our own reaction stops being expected
  const expectedEchoes = new Map<string, number>();
  let nextId = 0;

  function show(emoji: string): void {
    // Full: drop the newcomer. Evicting the oldest would make the burst flicker.
    if (floats.value.length >= MAX_FLOATS) return;
    const id = nextId++;
    floats.value = [...floats.value, { id, emoji, left: 8 + Math.round(random() * 84) }];
    const timer = setTimeout(() => {
      timers.delete(timer);
      floats.value = floats.value.filter((f) => f.id !== id);
    }, FLOAT_MS);
    timers.add(timer);
  }

  return {
    floats,
    receive(emoji) {
      const until = expectedEchoes.get(emoji);
      if (until !== undefined) {
        expectedEchoes.delete(emoji);
        if (Date.now() < until) return;
      }
      show(emoji);
    },
    own(emoji) {
      expectedEchoes.set(emoji, Date.now() + ECHO_MS);
      show(emoji);
    },
    dispose() {
      timers.forEach(clearTimeout);
      timers.clear();
      floats.value = [];
    },
  };
}
