import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

import {
  announceDownload,
  dismissDownloadNotice,
  DOWNLOAD_NOTICE_MS,
  downloadNoticeVisible,
} from "./download-notice";

describe("download notice", () => {
  beforeEach(() => vi.useFakeTimers());
  afterEach(() => {
    dismissDownloadNotice();
    vi.useRealTimers();
  });

  it("shows, then goes away by itself", () => {
    announceDownload();
    expect(downloadNoticeVisible.value).toBe(true);
    vi.advanceTimersByTime(DOWNLOAD_NOTICE_MS - 1);
    expect(downloadNoticeVisible.value).toBe(true);
    vi.advanceTimersByTime(1);
    expect(downloadNoticeVisible.value).toBe(false);
  });

  it("restarts the timer on a second download", () => {
    announceDownload();
    vi.advanceTimersByTime(DOWNLOAD_NOTICE_MS - 1000);
    announceDownload();
    vi.advanceTimersByTime(DOWNLOAD_NOTICE_MS - 1000);
    expect(downloadNoticeVisible.value).toBe(true);
    vi.advanceTimersByTime(1000);
    expect(downloadNoticeVisible.value).toBe(false);
  });

  it("can be dismissed", () => {
    announceDownload();
    dismissDownloadNotice();
    expect(downloadNoticeVisible.value).toBe(false);
  });
});
