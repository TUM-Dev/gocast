<script setup lang="ts">
import { BOOKMARK_ACTION, BOOKMARK_KEYS_LABEL, HOTKEY_HELP } from "@/lib/player/hotkeys";

defineProps<{ open: boolean }>();
const emit = defineEmits<{ close: [] }>();

// The rows come from the player's own bindings (lib/player/hotkeys.ts), so the popup
// cannot drift from what the keys do. Escape is the popup's, not the player's.
const SHORTCUTS: { action: string; keys: string }[] = [
  ...HOTKEY_HELP,
  { action: BOOKMARK_ACTION, keys: BOOKMARK_KEYS_LABEL },
  { action: "Skip silence", keys: "S / s" },
  { action: "Stats for nerds (player and management page)", keys: "I / i" },
  { action: "Close any kind of popup", keys: "ESCAPE" },
];
</script>

<template>
  <!--
    To <body> and above video.js: inside the watch page's action bar the popup shared a
    stacking context with the player, whose positioned controls painted over it (it had
    no z-index at all).
  -->
  <Teleport to="body">
    <div
      v-show="open"
      class="tum-live-popup-container z-50"
      role="dialog"
      aria-modal="true"
      aria-label="Keyboard shortcuts"
      @keydown.escape.window="emit('close')"
    >
      <div class="tum-live-popup tum-live-bg" @click.self="emit('close')">
        <h2>
          Keyboard Shortcuts
          <button type="button" class="tum-live-icon-button p-1" title="Close" @click="emit('close')">
            <i class="fa-solid fa-xmark"></i>
          </button>
        </h2>
        <ul>
          <li v-for="shortcut in SHORTCUTS" :key="shortcut.action">
            <strong>{{ shortcut.action }}</strong> <b>{{ shortcut.keys }}</b>
          </li>
        </ul>
      </div>
    </div>
  </Teleport>
</template>
