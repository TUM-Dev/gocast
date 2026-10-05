<script setup lang="ts">
import { computed, ref } from "vue";

/**
 * What a self-streaming lecturer enters in OBS or the like.
 *
 * The server-rendered card also showed the full ingest URL, built from the
 * deployment's IngestBase. The SPA has no way to learn that yet (getFrontendConfig
 * does not carry it), so only the key and its secret are shown; the URL's shape is
 * `<ingest base><stream key>?secret=<secret>`.
 */
const props = defineProps<{
  lectureId: number;
  courseSlug: string;
  streamKey: string;
  /** The hall change to self-streaming is not saved yet. */
  pending: boolean;
}>();

const shown = ref(false);
const copied = ref<"" | "key" | "secret">("");
let timer: ReturnType<typeof setTimeout> | undefined;

const key = computed(() => (props.courseSlug ? `${props.courseSlug}-${props.lectureId}` : ""));

async function copy(what: "key" | "secret", value: string): Promise<void> {
  try {
    await navigator.clipboard.writeText(value);
  } catch {
    return;
  }
  copied.value = what;
  clearTimeout(timer);
  timer = setTimeout(() => (copied.value = ""), 1500);
}
</script>

<template>
  <div class="text-3 rounded border p-2 text-sm dark:border-gray-700">
    <p v-if="pending" class="text-5 mb-1">Once saved, this lecture is streamed by you rather than from a hall.</p>
    <button type="button" class="text-5 hover:text-1" :aria-expanded="shown" @click="shown = !shown">
      <i class="fas fa-key mr-1"></i>{{ shown ? "Hide stream key" : "Show stream key" }}
    </button>
    <div v-if="shown" class="mt-2 flex flex-col gap-1">
      <p class="rounded bg-danger/25 p-2 text-xs">
        Self-streaming is experimental. Keep a local recording while you stream: it is the only copy
        if something fails while your stream is processed.
      </p>
      <p v-if="key" class="flex min-w-0 flex-wrap items-center gap-2">
        <span class="font-semibold">Stream key:</span>
        <code class="break-all">{{ key }}</code>
        <button type="button" class="text-5 hover:text-1" aria-label="Copy the stream key" @click="copy('key', key)">
          <i class="fas fa-clipboard"></i>
        </button>
        <span v-if="copied === 'key'" role="status" class="text-xs">Copied</span>
      </p>
      <p class="flex min-w-0 flex-wrap items-center gap-2">
        <span class="font-semibold">Secret:</span>
        <code class="break-all">{{ streamKey }}</code>
        <button
          type="button"
          class="text-5 hover:text-1"
          aria-label="Copy the secret"
          @click="copy('secret', streamKey)"
        >
          <i class="fas fa-clipboard"></i>
        </button>
        <span v-if="copied === 'secret'" role="status" class="text-xs">Copied</span>
      </p>
      <p class="text-5 text-xs">
        The stream server address is shown on the course page until this page can show it.
      </p>
    </div>
  </div>
</template>
