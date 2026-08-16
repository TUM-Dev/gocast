<script setup lang="ts">
defineProps<{ open: boolean }>();
const emit = defineEmits<{ close: [] }>();

// Copied from the shortcuts popup in home.gohtml; the bindings themselves live in the
// watch page's hotkey handling, which has not been migrated.
const SHORTCUTS: { action: string; keys: string }[] = [
  { action: "Toggle play/pause", keys: "K / k / SPACEBAR" },
  { action: "Mute/Unmute", keys: "M / m" },
  { action: "Open/Exit toggle fullscreen", keys: "F / f" },
  { action: "Seek forward", keys: "L / l / ArrowRight / Right" },
  { action: "Seek backwards", keys: "J / j / ArrowLeft / Left" },
  { action: "Volume up", keys: "ArrowUp / Up" },
  { action: "Volume down", keys: "ArrowDown / Down" },
  { action: "Increase playback rate", keys: "<" },
  { action: "Decrease playback rate", keys: ">" },
  {
    action: "Seek to specific point in the video (7 advances to 70% of duration)",
    keys: "0…9",
  },
  { action: "Skip silence", keys: "S / s" },
  { action: "Close any kind of popup", keys: "ESCAPE" },
];
</script>

<template>
  <div
    v-show="open"
    class="tum-live-popup-container"
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
</template>
