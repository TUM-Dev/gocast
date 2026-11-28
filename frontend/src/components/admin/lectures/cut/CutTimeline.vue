<script setup lang="ts">
import { nextTick, onBeforeUnmount, onMounted, ref, watch } from "vue";

import { columnLevels, formatTime, timeAtX, type Cut } from "@/lib/cuts";

/**
 * The recording laid out left to right with its audio levels, on which the cuts are made:
 * drag over a stretch to cut it out, drag the edges of a cut to adjust it, click to seek.
 * It only reports what the person did; the parent owns the cuts and the player.
 *
 * The waveform is a canvas, the rest (cuts, handles, playhead) is DOM positioned in
 * percent, so zooming is just a wider track and the pieces need no redrawing.
 */
const props = defineProps<{
  peaks: Uint8Array;
  bucketSeconds: number;
  duration: number;
  cuts: Cut[];
  /** Seconds. */
  currentTime: number;
  /** Where a cut was started with "start cut here" and is waiting for its end. */
  pendingStart: number | null;
  /** 1 fits the recording to the width, more makes the track wider and the box scrolls. */
  zoom: number;
  selectedId: number | null;
}>();

const emit = defineEmits<{
  seek: [seconds: number];
  create: [start: number, end: number];
  resize: [id: number, edge: "start" | "end", seconds: number];
  select: [id: number | null];
  remove: [id: number];
}>();

/** Less movement than this is a click. */
const CLICK_SLOP_PX = 4;
/** The finest a keyboard step seeks, and with shift. */
const KEY_STEP = 1;
const KEY_STEP_BIG = 10;

const scroller = ref<HTMLElement | null>(null);
const track = ref<HTMLElement | null>(null);
const canvas = ref<HTMLCanvasElement | null>(null);

const pct = (seconds: number) => (props.duration > 0 ? (Math.min(Math.max(seconds, 0), props.duration) / props.duration) * 100 : 0);

/* Drawing. */

function draw(): void {
  const el = canvas.value;
  const box = track.value;
  if (!el || !box) return;
  const cssWidth = box.clientWidth;
  const cssHeight = box.clientHeight;
  if (cssWidth <= 0 || cssHeight <= 0) return;
  // Browsers cap a canvas' size; a zoomed track is wide, so trade sharpness for it.
  const scale = Math.min(window.devicePixelRatio || 1, 16384 / cssWidth);
  el.width = Math.round(cssWidth * scale);
  el.height = Math.round(cssHeight * scale);
  const ctx = el.getContext("2d");
  if (!ctx) return;
  ctx.scale(scale, scale);
  ctx.clearRect(0, 0, cssWidth, cssHeight);
  ctx.fillStyle = getComputedStyle(el).color;

  const levels = columnLevels(props.peaks, props.bucketSeconds, props.duration, Math.floor(cssWidth));
  const mid = cssHeight / 2;
  for (let x = 0; x < levels.length; x++) {
    // At least a hairline, so silence reads as a line and not as a missing waveform.
    const h = Math.max(1, levels[x] * (cssHeight - 4));
    ctx.fillRect(x, mid - h / 2, 1, h);
  }
}

let observer: ResizeObserver | undefined;
onMounted(() => {
  draw();
  if (typeof ResizeObserver !== "undefined" && track.value) {
    observer = new ResizeObserver(() => draw());
    observer.observe(track.value);
  }
});
onBeforeUnmount(() => observer?.disconnect());
watch(() => [props.peaks, props.bucketSeconds, props.duration], draw);
watch(
  () => props.zoom,
  async () => {
    await nextTick();
    draw();
  },
);

/* Keeping the playhead in view while zoomed. */

let dragging = false;
watch(
  () => props.currentTime,
  (t) => {
    const view = scroller.value;
    const box = track.value;
    if (!view || !box || dragging || props.zoom <= 1) return;
    const x = (pct(t) / 100) * box.clientWidth;
    if (x < view.scrollLeft || x > view.scrollLeft + view.clientWidth) {
      view.scrollLeft = Math.max(0, x - view.clientWidth / 2);
    }
  },
);

/* Pointer. */

type Drag =
  | { kind: "make"; from: number; fromX: number; moved: boolean }
  | { kind: "edge"; id: number; edge: "start" | "end" };

const drag = ref<Drag | null>(null);
/** The stretch being dragged out, for the preview. */
const making = ref<{ start: number; end: number } | null>(null);
const hover = ref<number | null>(null);

function timeOf(e: PointerEvent): number {
  const rect = track.value!.getBoundingClientRect();
  return timeAtX(e.clientX - rect.left, rect.width, props.duration);
}

function onPointerDown(e: PointerEvent): void {
  if (!track.value || props.duration <= 0 || e.button > 0) return;
  const handle = (e.target as HTMLElement).closest<HTMLElement>("[data-handle]");
  if (handle) {
    drag.value = { kind: "edge", id: Number(handle.dataset.id), edge: handle.dataset.handle as "start" | "end" };
  } else {
    drag.value = { kind: "make", from: timeOf(e), fromX: e.clientX, moved: false };
  }
  dragging = true;
  track.value.setPointerCapture?.(e.pointerId);
  track.value.focus?.({ preventScroll: true });
}

function onPointerMove(e: PointerEvent): void {
  if (!track.value) return;
  hover.value = timeOf(e);
  const d = drag.value;
  if (!d) return;
  if (d.kind === "edge") {
    emit("resize", d.id, d.edge, timeOf(e));
    return;
  }
  if (!d.moved && Math.abs(e.clientX - d.fromX) < CLICK_SLOP_PX) return;
  d.moved = true;
  const to = timeOf(e);
  making.value = { start: Math.min(d.from, to), end: Math.max(d.from, to) };
}

function onPointerUp(e: PointerEvent): void {
  const d = drag.value;
  drag.value = null;
  dragging = false;
  track.value?.releasePointerCapture?.(e.pointerId);
  const made = making.value;
  making.value = null;
  if (!d || d.kind === "edge") return;
  if (d.moved && made) {
    emit("create", made.start, made.end);
    return;
  }
  const t = d.from;
  emit("seek", t);
  emit("select", props.cuts.find((c) => t >= c.start && t <= c.end)?.id ?? null);
}

function onKeyDown(e: KeyboardEvent): void {
  const step = e.shiftKey ? KEY_STEP_BIG : KEY_STEP;
  switch (e.key) {
    case "ArrowLeft":
      emit("seek", Math.max(0, props.currentTime - step));
      break;
    case "ArrowRight":
      emit("seek", Math.min(props.duration, props.currentTime + step));
      break;
    case "Delete":
    case "Backspace":
      if (props.selectedId === null) return;
      emit("remove", props.selectedId);
      break;
    default:
      return;
  }
  e.preventDefault();
}
</script>

<template>
  <div class="flex flex-col gap-1">
    <div ref="scroller" class="overflow-x-auto rounded-lg border border-gray-300 dark:border-gray-600">
      <div
        ref="track"
        class="relative h-28 touch-none select-none bg-white outline-none focus-visible:ring-2 focus-visible:ring-blue-500 dark:bg-gray-900"
        :style="{ width: `${Math.max(1, zoom) * 100}%` }"
        tabindex="0"
        role="application"
        aria-label="Audio timeline. Drag to cut out a stretch, click to seek, arrow keys step."
        data-testid="cut-timeline"
        @pointerdown="onPointerDown"
        @pointermove="onPointerMove"
        @pointerup="onPointerUp"
        @pointercancel="onPointerUp"
        @pointerleave="hover = null"
        @keydown="onKeyDown"
      >
        <canvas ref="canvas" class="absolute inset-0 h-full w-full text-blue-600 dark:text-blue-400" aria-hidden="true"></canvas>

        <div
          v-for="c in cuts"
          :key="c.id"
          class="absolute inset-y-0 border-x"
          :class="[
            c.suggested
              ? 'border-dashed border-amber-500 bg-amber-400/25'
              : 'border-red-600 bg-red-500/30',
            selectedId === c.id ? 'ring-2 ring-inset ring-blue-500' : '',
          ]"
          :style="{ left: `${pct(c.start)}%`, width: `${pct(c.end) - pct(c.start)}%` }"
          :data-testid="c.suggested ? 'cut-suggested' : 'cut-region'"
          :data-cut="c.id"
        >
          <span class="absolute inset-y-0 left-0 w-2 -translate-x-1/2 cursor-ew-resize" data-handle="start" :data-id="c.id"></span>
          <span class="absolute inset-y-0 right-0 w-2 translate-x-1/2 cursor-ew-resize" data-handle="end" :data-id="c.id"></span>
        </div>

        <div
          v-if="making"
          class="pointer-events-none absolute inset-y-0 bg-red-500/20 ring-1 ring-red-600"
          :style="{ left: `${pct(making.start)}%`, width: `${pct(making.end) - pct(making.start)}%` }"
          data-testid="cut-making"
        ></div>

        <div
          v-if="pendingStart !== null"
          class="pointer-events-none absolute inset-y-0 w-0.5 bg-red-600"
          :style="{ left: `${pct(pendingStart)}%` }"
          data-testid="cut-pending"
        ></div>

        <div
          class="pointer-events-none absolute inset-y-0 w-0.5 bg-gray-900 dark:bg-white"
          :style="{ left: `${pct(currentTime)}%` }"
          data-testid="cut-playhead"
        ></div>

        <div
          v-if="hover !== null && !drag"
          class="pointer-events-none absolute top-1 -translate-x-1/2 rounded bg-gray-900/80 px-1 text-xs text-white"
          :style="{ left: `${pct(hover)}%` }"
        >
          {{ formatTime(hover, true) }}
        </div>
      </div>
    </div>
    <div class="text-4 flex justify-between text-xs">
      <span>0:00</span>
      <span>{{ formatTime(duration) }}</span>
    </div>
  </div>
</template>
