<script setup lang="ts">
import { computed, onMounted, ref } from "vue";
import { fetchConfig } from "@/lib/config";

/**
 * What a self-streaming lecturer enters in OBS or the like.
 *
 * OBS joins its "Server" and "Stream key" fields with a slash, and the ingest
 * server's auth hook expects `secret=<secret>/<slug>-<id>` in the query. So the
 * server is `<ingest base><slug>-<id>?secret=<secret>` and the stream key repeats
 * `<slug>-<id>`. The base comes from getFrontendConfig; if it is not configured (or
 * the request fails) the server row is left out and the bare secret is shown instead.
 */
const props = defineProps<{
  lectureId: number;
  courseSlug: string;
  streamKey: string;
  /** The hall change to self-streaming is not saved yet. */
  pending: boolean;
}>();

const shown = ref(false);
const copied = ref<"" | "key" | "secret" | "server">("");
const ingestBase = ref("");
let timer: ReturnType<typeof setTimeout> | undefined;

const key = computed(() => (props.courseSlug ? `${props.courseSlug}-${props.lectureId}` : ""));

const server = computed(() =>
  ingestBase.value && key.value ? `${ingestBase.value}${key.value}?secret=${props.streamKey}` : "",
);

onMounted(async () => {
  try {
    ingestBase.value = (await fetchConfig()).ingestBase;
  } catch {
    // The key and secret are still shown.
  }
});

async function copy(what: "key" | "secret" | "server", value: string): Promise<void> {
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
      <p v-if="server" class="flex min-w-0 flex-wrap items-center gap-2">
        <span class="font-semibold">Server:</span>
        <code class="break-all">{{ server }}</code>
        <button type="button" class="text-5 hover:text-1" aria-label="Copy the server" @click="copy('server', server)">
          <i class="fas fa-clipboard"></i>
        </button>
        <span v-if="copied === 'server'" role="status" class="text-xs">Copied</span>
      </p>
      <p v-if="key" class="flex min-w-0 flex-wrap items-center gap-2">
        <span class="font-semibold">Stream key:</span>
        <code class="break-all">{{ key }}</code>
        <button type="button" class="text-5 hover:text-1" aria-label="Copy the stream key" @click="copy('key', key)">
          <i class="fas fa-clipboard"></i>
        </button>
        <span v-if="copied === 'key'" role="status" class="text-xs">Copied</span>
      </p>
      <p v-if="!server" class="flex min-w-0 flex-wrap items-center gap-2">
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
    </div>
  </div>
</template>
