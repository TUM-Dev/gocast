import { onBeforeUnmount, ref, watch, type Ref } from "vue";

/**
 * The element video.js draws the seekbar track in (`.vjs-progress-holder`), found inside
 * `container` once the player has built its controls. video.js loads on demand and
 * rebuilds its control bar, so this watches the subtree and re-resolves instead of
 * querying once. Overlays teleport into the returned element and so follow the track's
 * exact width, margins included, with no resize bookkeeping.
 */
export function useProgressHolder(container: Ref<Element | null | undefined>): Ref<Element | null> {
  const holder = ref<Element | null>(null);
  let observer: MutationObserver | undefined;

  const resolve = (root: Element) => {
    const found = root.querySelector(".vjs-progress-holder");
    if (found !== holder.value) holder.value = found;
  };

  watch(
    container,
    (root) => {
      observer?.disconnect();
      observer = undefined;
      holder.value = null;
      if (!root) return;
      resolve(root);
      observer = new MutationObserver(() => resolve(root));
      observer.observe(root, { childList: true, subtree: true });
    },
    { immediate: true, flush: "post" },
  );

  onBeforeUnmount(() => observer?.disconnect());
  return holder;
}
