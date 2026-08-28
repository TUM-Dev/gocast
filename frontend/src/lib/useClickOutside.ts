import { onBeforeUnmount, onMounted, type Ref } from "vue";

/**
 * Calls `handler` when a pointer press lands outside `target`.
 *
 * Replaces Alpine's `@click.outside`, which the templates use to dismiss the header
 * menus. Listening on pointerdown rather than click means a menu closes as the press
 * begins, matching how the legacy menus feel.
 */
export function useClickOutside(target: Ref<HTMLElement | null>, handler: () => void): void {
  function onPointerDown(event: PointerEvent): void {
    const el = target.value;
    if (el && event.target instanceof Node && !el.contains(event.target)) {
      handler();
    }
  }

  onMounted(() => document.addEventListener("pointerdown", onPointerDown));
  onBeforeUnmount(() => document.removeEventListener("pointerdown", onPointerDown));
}
