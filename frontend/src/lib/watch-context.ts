import { inject, type ComputedRef, type InjectionKey, type Ref, type ShallowRef } from "vue";

import type { PlayerController } from "./player/controller";
import type { Clip } from "./player/clip";
import type { SidebarTab, StreamWatch, WatchLayout, WatchPhase } from "./watch";

/**
 * What the player controller offers the page's regions: the video.js-independent
 * {@link PlayerController}, with times on the clip's clock, plus the conversion to the
 * recording's clock that silences, bookmarks, sections and transcript cues are in.
 */
export interface WatchController extends PlayerController {
  /** The recording's time for a time on the clip's clock (the same when uncut). */
  toRecordingTime(clipTime: number): number;
}

/** The sidebar's state; shared because buttons elsewhere (action bar, seekbar) open tabs. */
export interface WatchSidebarState {
  /** The open tab, or null when the sidebar is closed. */
  active: Readonly<Ref<SidebarTab | null>>;
  /** Tabs this stream and viewer have, in order. */
  tabs: ComputedRef<SidebarTab[]>;
  open(tab: SidebarTab): void;
  /** Opens the tab, or closes the sidebar if it is already the open one. */
  toggle(tab: SidebarTab): void;
  close(): void;
}

/**
 * Everything a region of the watch page may need, provided by WatchView. The regions
 * also receive the parts they need as explicit props or slot props; this is for deeper
 * descendants (and for regions added later) so nothing has to be threaded through.
 */
export interface WatchContext {
  /** The page data. Replaced (not mutated) by every refresh. */
  watch: Readonly<Ref<StreamWatch>>;
  streamId: ComputedRef<number>;
  slug: ComputedRef<string>;
  phase: ComputedRef<WatchPhase>;
  /** The layout in use; SPLIT means the split view, anything else one player. */
  layout: ComputedRef<WatchLayout>;
  /** Navigates to a layout (`/w/:slug/:id/COMB|PRES|CAM|SPLIT`), keeping `?t=` etc. */
  setLayout(layout: WatchLayout): void;
  /** The stored cut; the controller's clock starts at `clip.start` of the recording. */
  clip: ComputedRef<Clip | undefined>;
  /** The single player's controller; null while no player is mounted (waiting, split). */
  controller: Readonly<ShallowRef<WatchController | null>>;
  sidebar: WatchSidebarState;
  /** Fetches getStreamWatch again without blanking the page. */
  refresh(): Promise<void>;
}

export const watchContextKey: InjectionKey<WatchContext> = Symbol("watch-context");

export function useWatchContext(): WatchContext {
  const ctx = inject(watchContextKey);
  if (!ctx) throw new Error("useWatchContext() outside of WatchView");
  return ctx;
}
