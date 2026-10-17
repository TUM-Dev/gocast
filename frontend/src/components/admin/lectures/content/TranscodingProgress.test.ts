import { flushPromises, mount } from "@vue/test-utils";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

import TranscodingProgress from "@/components/admin/lectures/content/TranscodingProgress.vue";
import * as content from "@/lib/lecture-content";

const failed = { version: "PRES", progress: 0, failed: true, error: "The file contains no video." };
const running = { version: "COMB", progress: 40, failed: false, error: "" };

function show(converting: boolean, progresses: content.TranscodingProgress[]) {
  return mount(TranscodingProgress, { props: { courseId: 1, lectureId: 2, converting, progresses } });
}

describe("TranscodingProgress", () => {
  beforeEach(() => {
    vi.useFakeTimers();
  });
  afterEach(() => {
    vi.useRealTimers();
    vi.restoreAllMocks();
  });

  it("shows the percentage of a version under way", () => {
    const w = show(true, [running]);
    expect(w.text()).toContain("COMB (40%)");
    expect(w.find("[role=progressbar]").attributes("aria-valuenow")).toBe("40");
  });

  it("says why a conversion failed and how to go on, with no bar", () => {
    const w = show(false, [failed]);
    expect(w.find("[role=alert]").text()).toContain("PRES: conversion failed.");
    expect(w.text()).toContain("The file contains no video.");
    expect(w.text()).toContain("Upload the video again");
    expect(w.find("[role=progressbar]").exists()).toBe(false);
  });

  it("shows a failed version beside one still under way", () => {
    const w = show(true, [running, failed]);
    expect(w.find("[role=progressbar]").exists()).toBe(true);
    expect(w.find("[role=alert]").exists()).toBe(true);
  });

  it("shows nothing when nothing is listed", () => {
    expect(show(false, []).find("[aria-label='Transcoding progress']").exists()).toBe(false);
  });

  it("does not poll a lecture that only has a failure", async () => {
    const fetch = vi.spyOn(content, "fetchTranscodingProgress").mockResolvedValue([failed]);
    show(false, [failed]);
    await vi.advanceTimersByTimeAsync(content.PROGRESS_POLL_MS * 3);
    expect(fetch).not.toHaveBeenCalled();
  });

  it("keeps the failure on screen once the other version is done, and asks for a reload", async () => {
    vi.spyOn(content, "fetchTranscodingProgress").mockResolvedValue([failed]);
    const w = show(true, [running, failed]);

    await vi.advanceTimersByTimeAsync(content.PROGRESS_POLL_MS);
    await flushPromises();

    expect(w.emitted("done")).toHaveLength(1);
    expect(w.find("[role=progressbar]").exists()).toBe(false);
    expect(w.find("[role=alert]").text()).toContain("PRES: conversion failed.");
  });
});
