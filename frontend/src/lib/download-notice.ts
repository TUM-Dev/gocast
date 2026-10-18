import { readonly, ref } from "vue";

/**
 * How long the notice stays. The edge remuxes the recording before the first byte, which
 * takes a few seconds up to a minute for a long lecture, and the browser shows nothing
 * until then. A page can't observe a native download starting, so this is a timer, not a
 * promise that the download has begun.
 */
export const DOWNLOAD_NOTICE_MS = 15_000;

const visible = ref(false);
let timer: ReturnType<typeof setTimeout> | undefined;

export const downloadNoticeVisible = readonly(visible);

/** Call when the viewer starts a recording download; shows the "preparing" notice. */
export function announceDownload(): void {
  visible.value = true;
  clearTimeout(timer);
  timer = setTimeout(() => (visible.value = false), DOWNLOAD_NOTICE_MS);
}

export function dismissDownloadNotice(): void {
  clearTimeout(timer);
  visible.value = false;
}
