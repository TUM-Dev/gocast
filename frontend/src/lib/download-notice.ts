import { readonly, ref } from "vue";

/**
 * How long the notice stays. The edge remuxes the recording before the first byte, which
 * takes a few seconds up to a minute for a long lecture, and the browser shows nothing
 * until then. A page can't observe a native download starting, so this is a timer, not a
 * promise that the download has begun.
 */
export const DOWNLOAD_NOTICE_MS = 15_000;

const visible = ref(false);
const kind = ref<"preparing" | "busy">("preparing");
let timer: ReturnType<typeof setTimeout> | undefined;

export const downloadNoticeVisible = readonly(visible);
/** "busy" when the edge turned the download away because too many are running. */
export const downloadNoticeKind = readonly(kind);

function show(k: "preparing" | "busy"): void {
  kind.value = k;
  visible.value = true;
  clearTimeout(timer);
  timer = setTimeout(() => (visible.value = false), DOWNLOAD_NOTICE_MS);
}

/** Call when the viewer starts a recording download; shows the "preparing" notice. */
export function announceDownload(): void {
  show("preparing");
}

/** Shows that the edge rejected the download; the viewer should retry in a minute. */
export function announceDownloadBusy(): void {
  show("busy");
}

/**
 * Starts a recording download. The edge limits how many it prepares at once and answers
 * the rest with 503, but a rejected navigation would replace the watch page with the bare
 * error. So a probe asks first, and the browser only navigates when there is room. The
 * probe takes no slot, so the real request can still lose a race; that case ends on the
 * edge's plain error, which is rarer than the page being replaced every time.
 */
export async function startDownload(url: string): Promise<void> {
  announceDownload();
  try {
    const probe = new URL(url, window.location.href);
    probe.searchParams.set("probe", "1");
    const res = await fetch(probe.toString());
    if (res.status === 503) {
      announceDownloadBusy();
      return;
    }
  } catch {
    // The probe is only a courtesy: an unreachable edge or a missing CORS header must
    // not block a download the browser could make.
  }
  const a = document.createElement("a");
  a.href = url;
  a.download = "";
  a.click();
}

export function dismissDownloadNotice(): void {
  clearTimeout(timer);
  visible.value = false;
}
