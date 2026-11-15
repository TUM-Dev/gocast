import { defineStore } from "pinia";
import { computed, markRaw, ref, shallowRef } from "vue";

import type { CourseLecture } from "@/lib/course-lectures";
import { OPFSChunkStore, type ChunkStore } from "@/lib/recorder/chunk-store";
import { RecordingSession, type VersionStatus } from "@/lib/recorder/session";

/**
 * The lecture being recorded in this tab. A store rather than the wizard's state, so
 * that the recording and its upload go on whatever happens to the component that
 * started it, and the page can tell what leaving would cost.
 */
export const useBrowserRecordingStore = defineStore("browserRecording", () => {
  const opfs = markRaw(new OPFSChunkStore());
  const chunks: ChunkStore = opfs;
  const session = shallowRef<RecordingSession | null>(null);
  /**
   * What the session records, here rather than in the recorder's component: one that
   * mounts again, after the page was left during the upload, still has to know.
   */
  const lecture = shallowRef<CourseLecture | null>(null);
  const courseId = ref(0);
  /** The session's state, copied on every change so the template follows it. */
  const versions = ref<VersionStatus[]>([]);
  const storageError = ref("");
  const tick = ref(0);

  function refresh(): void {
    const s = session.value;
    versions.value = s ? s.status : [];
    storageError.value = s?.storageError ?? "";
    tick.value++;
  }

  function begin(course: number, recorded: CourseLecture): RecordingSession {
    const s = markRaw(new RecordingSession(chunks, refresh));
    session.value = s;
    courseId.value = course;
    lecture.value = recorded;
    refresh();
    return s;
  }

  function clear(): void {
    if (session.value?.busy) return;
    session.value = null;
    lecture.value = null;
    courseId.value = 0;
    refresh();
  }

  const recording = computed(() => {
    void tick.value;
    return session.value?.recording ?? false;
  });

  /** Recording, or chunks on their way. */
  const busy = computed(() => {
    void tick.value;
    return session.value?.busy ?? false;
  });

  // Closing or reloading the tab stops the recording and pauses the upload until the
  // course page is opened again; the browser asks first.
  window.addEventListener("beforeunload", (event) => {
    if (busy.value) event.preventDefault();
  });

  /** Whether recordings can be written to disk here; see OPFSChunkStore.probe. */
  async function storageWorks(): Promise<boolean> {
    try {
      await opfs.probe();
      return true;
    } catch {
      return false;
    }
  }

  return { chunks, storageWorks, session, lecture, courseId, versions, storageError, recording, busy, begin, clear };
});
