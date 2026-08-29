import { defineStore } from "pinia";
import { ref } from "vue";

/**
 * Whether the start page's sidebar is showing on a narrow screen.
 *
 * It is shared state because the button that opens it lives in the application
 * header and the sidebar it opens is inside the page, exactly as the `navigation`
 * toggle in web/ts/views/home.ts was shared between the two.
 *
 * On a wide screen the sidebar is always visible and this is ignored.
 */
export const useSidenavStore = defineStore("sidenav", () => {
  const open = ref(false);

  function toggle(value = !open.value): void {
    open.value = value;
  }

  function close(): void {
    open.value = false;
  }

  return { open, toggle, close };
});
