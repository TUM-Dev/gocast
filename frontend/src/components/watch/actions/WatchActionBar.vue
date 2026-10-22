<script setup lang="ts">
import { computed, ref } from "vue";

import KeyboardShortcuts from "@/components/KeyboardShortcuts.vue";
import { WatchLayout, type StreamWatch } from "@/gen/server/apiv2_pb";
import { announceDownload } from "@/lib/download-notice";
import type { PlayerController } from "@/lib/player/controller";
import {
  attachmentLink,
  copyablePlaylistUrl,
  layoutOptions,
  viewForLayout,
  type LayoutOption,
} from "@/lib/watch-actions";
import ActionMenu from "./ActionMenu.vue";
import ReactionsButton from "./ReactionsButton.vue";
import ReportIssueDialog from "./ReportIssueDialog.vue";
import ShareDialog from "./ShareDialog.vue";

/**
 * The row of buttons under the player: share, shortcuts, attachments and downloads,
 * copy HLS URL, view menu, report a problem and reactions.
 *
 * It owns no routing: choosing a layout emits `select-layout` and the shell changes the
 * route. Hidden in the waiting room by the shell (the old page did the same); this
 * component additionally drops what does not apply (no attachments, no downloads, ...).
 *
 * `controller` is only read for the current time (the share dialog). `layout` is the
 * layout on screen, to mark it in the menu and to pick which playlist "Copy HLS URL"
 * copies (none in SPLIT, where there are two).
 */
const props = defineProps<{
  streamId: number;
  views: StreamWatch["views"];
  splitAvailable: boolean;
  attachments: StreamWatch["attachments"];
  downloads: StreamWatch["downloads"];
  viewer: NonNullable<StreamWatch["viewer"]>;
  layout: WatchLayout;
  live: boolean;
  controller?: Pick<PlayerController, "getCurrentTime" | "getDuration">;
  /** The address to share; defaults to the current page. */
  pageUrl?: string;
  /** Prefills the report form's name. */
  userName?: string;
}>();
const emit = defineEmits<{ "select-layout": [option: LayoutOption] }>();

const shareOpen = ref(false);
const shortcutsOpen = ref(false);
const issueOpen = ref(false);
const copied = ref(false);
let copiedTimer: ReturnType<typeof setTimeout> | undefined;

// Read at render, which the dialog opening triggers, so a route change is not stale.
const currentHref = () => window.location.href;
const layouts = computed(() => layoutOptions(props.views, props.splitAvailable));
const attachmentLinks = computed(() => props.attachments.map((a) => attachmentLink(props.streamId, a)));
const currentLayout = computed(() => layouts.value.find((o) => o.layout === props.layout));
const playlist = computed(() => viewForLayout(props.views, props.layout)?.playlistUrl);

async function copyHls(): Promise<void> {
  if (!playlist.value) return;
  try {
    await navigator.clipboard.writeText(copyablePlaylistUrl(playlist.value));
    copied.value = true;
    clearTimeout(copiedTimer);
    copiedTimer = setTimeout(() => (copied.value = false), 1000);
  } catch {
    // Clipboard access can be denied; nothing useful to tell the viewer.
  }
}

const itemClass =
  "text-3 flex items-center gap-2 rounded px-2 py-1 text-left text-xs font-semibold transition-colors hover:bg-gray-200 dark:hover:bg-gray-600";
// Kept in step with the trigger in ActionMenu and the one in ReactionsButton.
const buttonClass =
  "group text-4 hover:text-1 flex h-8 w-8 items-center justify-center rounded transition-colors hover:bg-black/5 active:scale-95 dark:hover:bg-white/10";
const iconClass = "fa-solid text-lg transition-transform motion-safe:group-hover:scale-110";
</script>

<template>
  <div
    class="flex h-fit w-fit items-center gap-1 whitespace-nowrap rounded-lg bg-gray-100 px-3 py-1 dark:bg-secondary-light"
    role="toolbar"
    aria-label="Stream actions"
    data-testid="watch-action-bar"
  >
    <button type="button" :class="buttonClass" title="Share" aria-label="Share" @click="shareOpen = true">
      <i :class="[iconClass, 'fa-share-nodes']"></i>
    </button>

    <ActionMenu v-if="attachmentLinks.length > 0 || downloads.length > 0" label="Attachments and downloads" icon="fa-file-arrow-down">
      <a
        v-for="a in attachmentLinks"
        :key="a.href"
        role="menuitem"
        :href="a.href"
        :class="itemClass"
        :target="a.external ? '_blank' : undefined"
        :rel="a.external ? 'noopener noreferrer' : undefined"
        :download="a.external ? undefined : ''"
      >
        <i class="fa-solid fa-file text-xs text-gray-500 dark:text-gray-200"></i>
        <span class="truncate">{{ a.label }}</span>
      </a>
      <a
        v-for="d in downloads"
        :key="d.url"
        role="menuitem"
        :href="d.url"
        download
        :class="itemClass"
        @click="announceDownload"
      >
        <i class="fa-solid fa-download text-xs text-gray-500 dark:text-gray-200"></i>
        <span>Download {{ d.friendlyName }}</span>
      </a>
    </ActionMenu>

    <ActionMenu
      v-if="layouts.length > 1"
      label="Change view"
      :button-label="currentLayout ? `Change view, current: ${currentLayout.label}` : 'Change view'"
      :text="currentLayout?.label"
      icon="fa-camera-rotate"
    >
      <button
        v-for="o in layouts"
        :key="o.segment"
        type="button"
        role="menuitemradio"
        :aria-checked="o.layout === layout"
        :class="[itemClass, o.layout === layout && 'bg-gray-200 dark:bg-gray-600']"
        @click="emit('select-layout', o)"
      >
        {{ o.label }}
      </button>
    </ActionMenu>

    <button
      v-if="playlist"
      type="button"
      :class="buttonClass"
      title="Copy HLS URL"
      aria-label="Copy HLS URL"
      @click="copyHls"
    >
      <i :class="[iconClass, copied ? 'fa-check' : 'fa-link']"></i>
    </button>

    <button
      type="button"
      :class="buttonClass"
      title="Keyboard shortcuts"
      aria-label="Keyboard shortcuts"
      @click="shortcutsOpen = true"
    >
      <i :class="[iconClass, 'fa-keyboard']"></i>
    </button>

    <button
      v-if="viewer.canReportIssue"
      type="button"
      :class="buttonClass"
      title="Issues with the stream? Report a problem"
      aria-label="Report a problem"
      @click="issueOpen = true"
    >
      <i :class="[iconClass, 'fa-triangle-exclamation']"></i>
    </button>

    <ReactionsButton v-if="live && viewer.signedIn" :stream-id="streamId" />
  </div>

  <ShareDialog
    :open="shareOpen"
    :page-url="pageUrl ?? currentHref()"
    :live="live"
    :current-time="() => controller?.getCurrentTime() ?? 0"
    :duration="() => controller?.getDuration() ?? NaN"
    @close="shareOpen = false"
  />
  <KeyboardShortcuts :open="shortcutsOpen" @close="shortcutsOpen = false" />
  <ReportIssueDialog
    v-if="viewer.canReportIssue"
    :open="issueOpen"
    :stream-id="streamId"
    :default-name="userName"
    @close="issueOpen = false"
  />
</template>
