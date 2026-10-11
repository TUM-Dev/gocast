<script setup lang="ts">
/**
 * The watch page's skeleton: where each region goes, nothing about what is in it.
 *
 * Regions (named slots), top to bottom in the main column, then the side column:
 *  - `player`      the picture: one WatchPlayer, or the split view.
 *  - `admin-strip` course administrators' controls under the picture (camera presets,
 *                  end stream, ...). Rendered only when the slot is filled.
 *  - `info`        title, course, status line (WatchInfo).
 *  - `action-bar`  the buttons: layout switch, bookmark, share, shortcuts, reactions...
 *  - `description` the lecture's description.
 *  - `sidebar`     the tabbed side panel (WatchSidebar); shown beside the main column
 *                  from `lg` up and below it on a phone, when `sidebarOpen`. It stays mounted while closed (hidden), so the chat keeps its
 *                  socket and the bookmarks keep feeding the seekbar.
 *
 * On a phone the picture sticks to the top while the rest scrolls, as the old page did.
 */
defineProps<{ sidebarOpen: boolean }>();
</script>

<template>
  <div class="flex w-full max-w-[1800px] flex-col gap-4 lg:flex-row lg:items-start lg:p-4" data-testid="watch-layout">
    <div class="flex min-w-0 grow flex-col">
      <!-- On a wide screen a full-width 16:9 picture is taller than the window and
           pushes the title below the fold, so it is capped at what fits, centred on black
           like a theatre mode. The phone's picture is as wide as the screen. -->
      <div class="sticky top-0 z-30 lg:static lg:bg-black" data-testid="region-player">
        <div class="mx-auto lg:max-w-[calc((100vh-9rem)*16/9)]">
          <slot name="player" />
        </div>
      </div>
      <div v-if="$slots['admin-strip']" data-testid="region-admin-strip">
        <slot name="admin-strip" />
      </div>
      <div class="px-4 pt-4 lg:px-1">
        <slot name="info" />
        <div v-if="$slots['action-bar']" class="pt-3" data-testid="region-action-bar">
          <slot name="action-bar" />
        </div>
        <div v-if="$slots.description" class="pt-4" data-testid="region-description">
          <slot name="description" />
        </div>
      </div>
    </div>
    <aside
      v-if="$slots.sidebar"
      v-show="sidebarOpen"
      class="h-[70vh] w-full shrink-0 border-t lg:sticky lg:top-4 lg:h-[calc(100vh-7rem)] lg:w-[26rem] lg:rounded-lg lg:border dark:border-gray-800"
      data-testid="region-sidebar"
    >
      <slot name="sidebar" />
    </aside>
  </div>
</template>
