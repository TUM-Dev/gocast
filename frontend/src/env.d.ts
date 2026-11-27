/// <reference types="vite/client" />
/// <reference types="vite-plugin-pwa/client" />

/** Whether the service worker runs under `vite dev`; set in vite.config.ts. */
declare const __PWA_DEV__: boolean;

declare module "*.vue" {
  import type { DefineComponent } from "vue";

  const component: DefineComponent<Record<string, never>, Record<string, never>, unknown>;
  export default component;
}

// Neither ships types. The sprite plugin registers itself on import; the AirPlay one
// exports a function that takes video.js (see lib/player/airplay.ts).
declare module "videojs-sprite-thumbnails";
declare module "@silvermine/videojs-airplay" {
  const register: (videojs: unknown) => void;
  export default register;
}
