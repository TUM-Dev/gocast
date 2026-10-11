/// <reference types="vite/client" />

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
