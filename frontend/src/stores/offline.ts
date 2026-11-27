import { defineStore } from "pinia";
import { ref, shallowRef } from "vue";

import { ApiError } from "@/lib/api";
import {
  completeOfflineDownload,
  deleteOfflineLecture,
  requestOfflineLecture,
  syncOfflineLicenses,
} from "@/lib/offline/api";
import { deleteLecture, listLectures, putLecture, type OfflineLectureRecord } from "@/lib/offline/db";
import { downloadPackage, ensureSpace, OfflineError, recordFromLecture, waitUntilReady } from "@/lib/offline/download";
import { deletePackage } from "@/lib/offline/media";

/**
 * The lectures saved on this device: what the offline page lists, what the save
 * buttons show, and what the watch page plays from the device instead of the network.
 * The records live in IndexedDB; this mirrors them for the page.
 */
export const useOfflineStore = defineStore("offline", () => {
  // Replaced, never mutated, like the watch store's answer.
  const lectures = shallowRef<OfflineLectureRecord[]>([]);
  /** Download progress of lectures being saved, 0 to 1, by stream. */
  const progress = ref<Record<number, number>>({});
  /** The last error saving a lecture, by stream. */
  const errors = ref<Record<number, string>>({});
  const saving = ref<Record<number, boolean>>({});
  /**
   * The last save that ended, for the app-wide notice (OfflineSaveNotice.vue): the menu
   * the save button sits in closes on the click, taking the button's own label with it.
   * The title is kept here because a failed lecture's record is gone by then.
   */
  const finished = ref<{ streamId: number; title: string; error?: string } | null>(null);

  let loading: Promise<void> | null = null;

  /** Reads the saved lectures from the device. Only once: later changes go through here. */
  function load(): Promise<void> {
    loading ??= listLectures()
      .then((all) => {
        lectures.value = all.sort((a, b) => b.savedAt - a.savedAt);
      })
      .catch(() => {
        // No IndexedDB (a private window, blocked site data): nothing is saved, and
        // saving will say why when it fails.
        loading = null;
      });
    return loading;
  }

  function byStream(streamId: number): OfflineLectureRecord | undefined {
    return lectures.value.find((l) => l.streamId === streamId);
  }

  function upsert(record: OfflineLectureRecord) {
    lectures.value = [record, ...lectures.value.filter((l) => l.streamId !== record.streamId)];
  }

  function drop(streamId: number) {
    lectures.value = lectures.value.filter((l) => l.streamId !== streamId);
  }

  async function forget(record: OfflineLectureRecord) {
    await deletePackage(record.packageId);
    await deleteLecture(record.streamId);
    drop(record.streamId);
  }

  /**
   * Saves a lecture, or resumes saving it: a lecture still being prepared or downloaded
   * picks up where it stopped, so leaving the page costs only what was in flight.
   */
  async function save(streamId: number): Promise<void> {
    if (saving.value[streamId]) return;
    saving.value = { ...saving.value, [streamId]: true };
    const { [streamId]: _, ...rest } = errors.value;
    errors.value = rest;
    try {
      await load();
      let record = byStream(streamId);
      if (record?.state === "saved") return;
      if (!record) {
        record = recordFromLecture(await requestOfflineLecture(streamId));
        await putLecture(record);
        upsert(record);
      }
      const ready = await waitUntilReady(record.packageId);
      await ensureSpace(Number(ready.sizeBytes));
      record = await downloadPackage(record, ready, (p) => (progress.value = { ...progress.value, [streamId]: p }));
      upsert(record);
      await complete(record);
      finished.value = { streamId, title: record.title };
    } catch (err) {
      const error = messageOf(err);
      errors.value = { ...errors.value, [streamId]: error };
      const record = byStream(streamId);
      finished.value = { streamId, title: record?.title ?? "", error };
      if (err instanceof OfflineError && err.final && record) await forget(record);
    } finally {
      const { [streamId]: _p, ...p } = progress.value;
      progress.value = p;
      const { [streamId]: _s, ...s } = saving.value;
      saving.value = s;
    }
  }

  /** Tells the server the download is complete, so it deletes its copy. */
  async function complete(record: OfflineLectureRecord): Promise<void> {
    try {
      await completeOfflineDownload(record.packageId);
    } catch {
      return; // the next sync tries again
    }
    const done = { ...record, completed: true };
    await putLecture(done);
    upsert(done);
  }

  /** Removes a lecture from the device, and gives up its licence on the server. */
  async function remove(streamId: number): Promise<void> {
    const record = byStream(streamId);
    if (!record) return;
    await forget(record);
    deleteOfflineLecture(record.packageId).catch(() => {
      // Offline, or signed out: the licence runs out on its own.
    });
  }

  /**
   * Renews the licences of everything saved, and removes what the viewer may no longer
   * have. Needs the network and a session; without either it leaves things as they are,
   * and the licences run out in their own time.
   */
  async function sync(): Promise<void> {
    if (typeof navigator !== "undefined" && !navigator.onLine) return;
    await load();
    const held = lectures.value.filter((l) => l.state === "saved");
    if (held.length === 0) return;

    for (const l of held.filter((l) => !l.completed)) await complete(l);

    let licences: Map<number, number | null>;
    try {
      licences = await syncOfflineLicenses(held.map((l) => l.packageId));
    } catch (err) {
      if (err instanceof ApiError && err.isUnauthenticated) return;
      throw err;
    }
    for (const l of held) {
      const expiresAt = licences.get(l.packageId);
      if (expiresAt === undefined) continue;
      if (expiresAt === null) {
        await forget(l);
        continue;
      }
      const renewed = { ...byStream(l.streamId)!, expiresAt };
      await putLecture(renewed);
      upsert(renewed);
    }
  }

  return { lectures, progress, errors, saving, finished, load, byStream, save, remove, sync };
});

function messageOf(err: unknown): string {
  if (err instanceof OfflineError) return err.message;
  if (err instanceof ApiError) {
    if (err.status === 429 || err.status === 503 || err.status === 403) return err.message;
    if (err.isUnauthenticated) return "Sign in to save lectures offline.";
  }
  if (typeof navigator !== "undefined" && !navigator.onLine) return "You're offline. Save the lecture again once you're back online.";
  return "The lecture couldn't be saved. Please try again.";
}
