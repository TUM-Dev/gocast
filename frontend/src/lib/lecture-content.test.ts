import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

import { ApiError, clearToken } from "./api";
import {
  attachmentsOf,
  checkSection,
  contentErrorMessage,
  createLectureSections,
  customThumbnailOf,
  deleteLectureAttachment,
  deleteLectureSection,
  deleteLectureThumbnail,
  downloadsOf,
  fetchTranscodingProgress,
  fileUrl,
  formatSectionStart,
  pollTranscodingProgress,
  requestLectureSubtitles,
  sectionChanged,
  shouldPollProgress,
  subtitleErrorMessage,
  updateLectureSection,
  uploadLectureAttachment,
  uploadProblem,
  type TranscodingProgress,
} from "./lecture-content";

let fetchMock: ReturnType<typeof vi.fn>;

/** Answers the token request, then the given body for everything else. */
function respondWith(body: unknown, status = 200): void {
  fetchMock.mockImplementation((url: string) => {
    if (String(url).endsWith("/auth/token")) {
      return Promise.resolve(new Response(JSON.stringify({ access_token: "t", expires_in: 900 }), { status: 200 }));
    }
    return Promise.resolve(new Response(JSON.stringify(body), { status }));
  });
}

const lastCall = () => fetchMock.mock.calls.at(-1) as [string, RequestInit];

beforeEach(() => {
  clearToken();
  fetchMock = vi.fn();
  vi.stubGlobal("fetch", fetchMock);
});

afterEach(() => {
  vi.unstubAllGlobals();
  vi.useRealTimers();
});

describe("checkSection", () => {
  it("takes empty time fields as 0 and trims the description", () => {
    expect(checkSection({ hours: "", minutes: 4, seconds: "2", description: "  Intro " })).toEqual({
      ok: true,
      input: { description: "Intro", startHours: 0, startMinutes: 4, startSeconds: 2 },
    });
  });

  it("requires a description, as the RPC does", () => {
    const check = checkSection({ hours: 0, minutes: 0, seconds: 0, description: "   " });
    expect(check).toEqual({ ok: false, problems: { description: "Give the section a description." } });
  });

  it("refuses minutes or seconds of 60 and more", () => {
    for (const [minutes, seconds] of [
      [60, 0],
      [0, 60],
    ]) {
      const check = checkSection({ hours: 0, minutes, seconds, description: "x" });
      expect(check.ok).toBe(false);
      if (!check.ok) expect(check.problems.time).toBe("Minutes and seconds must be below 60.");
    }
  });

  it("allows any number of hours", () => {
    expect(checkSection({ hours: 40, minutes: 59, seconds: 59, description: "x" }).ok).toBe(true);
  });

  it("refuses fractions and negatives", () => {
    for (const bad of [{ minutes: 1.5 }, { seconds: -1 }, { hours: "1e2" }, { minutes: "-3" }]) {
      const check = checkSection({ hours: 0, minutes: 0, seconds: 0, description: "x", ...bad });
      expect(check.ok, JSON.stringify(bad)).toBe(false);
    }
  });

  it("reports both problems at once", () => {
    const check = checkSection({ hours: 0, minutes: 70, seconds: 0, description: "" });
    expect(check.ok).toBe(false);
    if (!check.ok) expect(Object.keys(check.problems).sort()).toEqual(["description", "time"]);
  });
});

describe("formatSectionStart", () => {
  it("leaves out the hours when there are none", () => {
    expect(formatSectionStart({ startHours: 0, startMinutes: 4, startSeconds: 2 })).toBe("4:02");
    expect(formatSectionStart({ startHours: 1, startMinutes: 4, startSeconds: 2 })).toBe("1:04:02");
    expect(formatSectionStart({ startHours: 0, startMinutes: 0, startSeconds: 0 })).toBe("0:00");
  });
});

describe("sectionChanged", () => {
  const section = { id: 1, description: "Intro", startHours: 0, startMinutes: 1, startSeconds: 0, fileId: 0 };

  it("compares description and start", () => {
    expect(sectionChanged(section, { description: "Intro", startHours: 0, startMinutes: 1, startSeconds: 0 })).toBe(
      false,
    );
    expect(sectionChanged(section, { description: "Intro", startHours: 0, startMinutes: 0, startSeconds: 0 })).toBe(
      true,
    );
  });
});

describe("files", () => {
  const files = [
    { id: 1, type: 1, friendlyName: "comb.mp4" },
    { id: 2, type: 2, friendlyName: "slides.pdf" },
    { id: 3, type: 4, friendlyName: "thumb" },
    { id: 4, type: 11, friendlyName: "cover.png" },
  ];

  it("sorts the list's files into attachments, the custom thumbnail and downloads", () => {
    expect(attachmentsOf(files).map((f) => f.id)).toEqual([2]);
    expect(customThumbnailOf(files)?.id).toBe(4);
    expect(customThumbnailOf(files.slice(0, 3))).toBeUndefined();
    expect(downloadsOf(files).map((f) => f.id)).toEqual([1]);
  });

  it("links them through the download endpoint", () => {
    expect(fileUrl(4, "serve")).toBe("/api/download/4?type=serve");
    expect(fileUrl(2, "download")).toBe("/api/download/2?type=download");
  });

  it("refuses an oversized file or a thumbnail that is not an image before sending it", () => {
    const big = new File(["x"], "big.pdf");
    Object.defineProperty(big, "size", { value: 50 * 1000 * 1000 + 1 });
    expect(uploadProblem(big, "attachment")).toMatch(/too large/);
    expect(uploadProblem(new File(["x"], "slides.pdf"), "attachment")).toBe("");
    expect(uploadProblem(new File(["x"], "cover.pdf", { type: "application/pdf" }), "thumbnail")).toMatch(/image/);
    expect(uploadProblem(new File(["x"], "cover.bmp", { type: "image/bmp" }), "thumbnail")).toMatch(/image/);
    expect(uploadProblem(new File(["x"], "Cover.PNG", { type: "image/png" }), "thumbnail")).toBe("");
  });
});

describe("the content calls", () => {
  it("create sections and read back the IDs", async () => {
    respondWith({ sections: [{ id: 5, description: "Intro" }, { id: 6, description: "Hops", startMinutes: 3 }] });

    const ids = await createLectureSections(1, 2, [
      { description: "Hops", startHours: 0, startMinutes: 3, startSeconds: 0 },
    ]);

    expect(ids).toEqual([5, 6]);
    const [url, init] = lastCall();
    expect(url).toBe("/api/v2/courses/1/streams/2/sections");
    expect(init.method).toBe("POST");
    expect(JSON.parse(init.body as string)).toEqual({
      sections: [{ description: "Hops", startHours: 0, startMinutes: 3, startSeconds: 0 }],
    });
  });

  it("update a section with zero values sent, not dropped", async () => {
    respondWith({});
    await updateLectureSection(1, 2, 5, { description: "Intro", startHours: 0, startMinutes: 0, startSeconds: 0 });
    const [url, init] = lastCall();
    expect(url).toBe("/api/v2/courses/1/streams/2/sections/5");
    expect(init.method).toBe("PUT");
    expect(JSON.parse(init.body as string)).toEqual({
      description: "Intro",
      startHours: 0,
      startMinutes: 0,
      startSeconds: 0,
    });
  });

  it("delete sections, attachments and the thumbnail at their paths", async () => {
    respondWith({});
    await deleteLectureSection(1, 2, 5);
    expect(lastCall()).toEqual(["/api/v2/courses/1/streams/2/sections/5", expect.objectContaining({ method: "DELETE" })]);
    await deleteLectureAttachment(1, 2, 9);
    expect(lastCall()[0]).toBe("/api/v2/courses/1/streams/2/attachments/9");
    await deleteLectureThumbnail(1, 2);
    expect(lastCall()[0]).toBe("/api/v2/courses/1/streams/2/thumbnail");
  });

  it("upload an attachment as the multipart field `file`", async () => {
    respondWith({ id: 9, type: 2, friendlyName: "slides.pdf" });
    const file = new File(["%PDF"], "slides.pdf", { type: "application/pdf" });

    expect(await uploadLectureAttachment(1, 2, file)).toEqual({ id: 9, type: 2, friendlyName: "slides.pdf" });
    const [url, init] = lastCall();
    expect(url).toBe("/api/v2/courses/1/streams/2/attachments");
    expect((init.body as FormData).get("file")).toBeInstanceOf(File);
  });

  it("request subtitles in a language", async () => {
    respondWith({});
    await requestLectureSubtitles(1, 2, "en");
    const [url, init] = lastCall();
    expect(url).toBe("/api/v2/courses/1/streams/2/subtitles");
    expect(JSON.parse(init.body as string)).toEqual({ language: "en" });
  });

  it("read the progress, an empty answer as nothing transcoding", async () => {
    respondWith({ progresses: [{ version: "CAM", progress: 40 }] });
    expect(await fetchTranscodingProgress(1, 2)).toEqual([{ version: "CAM", progress: 40 }]);
    expect(lastCall()[0]).toBe("/api/v2/courses/1/streams/2/transcoding-progress");

    respondWith({});
    expect(await fetchTranscodingProgress(1, 2)).toEqual([]);
  });
});

describe("shouldPollProgress", () => {
  it("polls only a converting lecture with a version still listed", () => {
    expect(shouldPollProgress(true, [{ version: "COMB", progress: 3 }])).toBe(true);
    expect(shouldPollProgress(true, [])).toBe(false);
    expect(shouldPollProgress(false, [{ version: "COMB", progress: 3 }])).toBe(false);
  });
});

describe("pollTranscodingProgress", () => {
  beforeEach(() => {
    vi.useFakeTimers();
  });

  function poller(answers: (TranscodingProgress[] | Error)[]) {
    const fetch = vi.fn(() => {
      const next = answers.shift() ?? [];
      return next instanceof Error ? Promise.reject(next) : Promise.resolve(next);
    });
    const onProgress = vi.fn();
    const onDone = vi.fn();
    const stop = pollTranscodingProgress({ fetch, onProgress, onDone, intervalMs: 1000 });
    return { fetch, onProgress, onDone, stop };
  }

  it("asks every interval until nothing is transcoding, then stops", async () => {
    const p = poller([[{ version: "COMB", progress: 10 }], [{ version: "COMB", progress: 60 }], []]);

    expect(p.fetch).not.toHaveBeenCalled();
    await vi.advanceTimersByTimeAsync(1000);
    expect(p.onProgress).toHaveBeenLastCalledWith([{ version: "COMB", progress: 10 }]);
    await vi.advanceTimersByTimeAsync(1000);
    expect(p.onProgress).toHaveBeenLastCalledWith([{ version: "COMB", progress: 60 }]);
    await vi.advanceTimersByTimeAsync(1000);
    expect(p.onDone).toHaveBeenCalledOnce();

    await vi.advanceTimersByTimeAsync(5000);
    expect(p.fetch).toHaveBeenCalledTimes(3);
  });

  it("retries after a failure but gives up on a lecture that is gone", async () => {
    const p = poller([new ApiError(500, "boom"), new ApiError(404, "gone")]);
    await vi.advanceTimersByTimeAsync(1000);
    await vi.advanceTimersByTimeAsync(1000);
    await vi.advanceTimersByTimeAsync(5000);
    expect(p.fetch).toHaveBeenCalledTimes(2);
    expect(p.onDone).not.toHaveBeenCalled();
  });

  it("stops when told to", async () => {
    const p = poller([[{ version: "COMB", progress: 10 }]]);
    p.stop();
    await vi.advanceTimersByTimeAsync(5000);
    expect(p.fetch).not.toHaveBeenCalled();
  });
});

describe("error messages", () => {
  it("say subtitles are unavailable on a 503", () => {
    expect(subtitleErrorMessage(new ApiError(503, "no voice service configured"))).toBe(
      "Subtitle generation is not available right now.",
    );
    expect(subtitleErrorMessage(new ApiError(400, "the lecture has no recording"))).toBe(
      "This lecture has no recording to generate subtitles from.",
    );
  });

  it("pass the server's validation message through", () => {
    expect(contentErrorMessage(new ApiError(400, "minutes and seconds must be below 60"))).toBe(
      "minutes and seconds must be below 60",
    );
    expect(contentErrorMessage(new ApiError(404, "no such attachment"))).toMatch(/no longer exists/);
    expect(contentErrorMessage(new Error("x"))).toBe("Something went wrong. Please try again.");
  });
});
