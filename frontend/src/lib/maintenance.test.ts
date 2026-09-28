import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

import { clearToken } from "./api";
import {
  deleteEmailFailure,
  deleteTranscodingFailure,
  fetchCronJobs,
  fetchEmailFailures,
  fetchThumbnailStatus,
  fetchTranscodingFailures,
  generateThumbnails,
  runCronJob,
} from "./maintenance";

let fetchMock: ReturnType<typeof vi.fn>;

/** Answers the token request, then the given body for everything else. */
function respondWith(body: unknown, status = 200): void {
  fetchMock.mockImplementation((url: string) => {
    if (String(url).endsWith("/auth/token")) {
      return Promise.resolve(
        new Response(JSON.stringify({ access_token: "t", expires_in: 900 }), { status: 200 }),
      );
    }
    return Promise.resolve(new Response(JSON.stringify(body), { status }));
  });
}

beforeEach(() => {
  clearToken();
  fetchMock = vi.fn();
  vi.stubGlobal("fetch", fetchMock);
});

afterEach(() => {
  vi.unstubAllGlobals();
});

describe("fetchThumbnailStatus", () => {
  it("reads running and progress off the wire", async () => {
    respondWith({ running: true, progress: 0.42 });

    const status = await fetchThumbnailStatus();

    expect(status).toEqual({ running: true, progress: 0.42 });
  });

  it("reads an idle job as not running with zero progress", async () => {
    // protojson omits false/zero fields, so an idle job answers `{}`.
    respondWith({});

    const status = await fetchThumbnailStatus();

    expect(status).toEqual({ running: false, progress: 0 });
  });
});

describe("generateThumbnails", () => {
  it("posts to the generate endpoint and returns the resulting status", async () => {
    respondWith({ running: true, progress: 0 });

    const status = await generateThumbnails();

    expect(status.running).toBe(true);
    const [url, init] = fetchMock.mock.calls.at(-1) as [string, RequestInit];
    expect(url).toBe("/api/v2/admin/maintenance/thumbnails/generate");
    expect(init.method).toBe("POST");
  });
});

describe("fetchCronJobs", () => {
  it("lists the registered job names", async () => {
    respondWith({ jobs: ["fetchCourses", "collectStats"] });

    await expect(fetchCronJobs()).resolves.toEqual(["fetchCourses", "collectStats"]);
  });

  it("reads no jobs as an empty list rather than a failure", async () => {
    respondWith({});

    await expect(fetchCronJobs()).resolves.toEqual([]);
  });
});

describe("runCronJob", () => {
  it("posts the job name as the request body", async () => {
    respondWith({});

    await runCronJob("fetchCourses");

    const [url, init] = fetchMock.mock.calls.at(-1) as [string, RequestInit];
    expect(url).toBe("/api/v2/admin/maintenance/cron/run");
    expect(init.method).toBe("POST");
    expect(JSON.parse(init.body as string)).toEqual({ job: "fetchCourses" });
  });
});

describe("fetchTranscodingFailures", () => {
  it("reads every field off the wire", async () => {
    respondWith({
      failures: [
        {
          id: 1,
          streamId: 7,
          version: "COMB",
          friendlyTime: "01.01.2026 00:00",
          hostname: "worker-1",
          filePath: "/rec/1.mp4",
          logs: "boom",
        },
      ],
    });

    const [failure] = await fetchTranscodingFailures();

    expect(failure).toEqual({
      id: 1,
      streamId: 7,
      version: "COMB",
      friendlyTime: "01.01.2026 00:00",
      hostname: "worker-1",
      filePath: "/rec/1.mp4",
      logs: "boom",
    });
  });

  it("reads an empty list as no failures rather than as a failure", async () => {
    respondWith({});

    await expect(fetchTranscodingFailures()).resolves.toEqual([]);
  });
});

describe("deleteTranscodingFailure", () => {
  it("deletes by id", async () => {
    respondWith({});

    await deleteTranscodingFailure(3);

    const [url, init] = fetchMock.mock.calls.at(-1) as [string, RequestInit];
    expect(url).toBe("/api/v2/admin/maintenance/transcoding-failures/3");
    expect(init.method).toBe("DELETE");
  });
});

describe("fetchEmailFailures", () => {
  it("reads a last-try timestamp as a Date", async () => {
    respondWith({
      failures: [
        {
          id: 1,
          to: "a@b.de",
          subject: "s",
          body: "b",
          lastTry: "2026-01-02T03:04:05Z",
          retries: 3,
          errors: "boom",
        },
      ],
    });

    const [failure] = await fetchEmailFailures();

    expect(failure.lastTry).toEqual(new Date("2026-01-02T03:04:05Z"));
    expect(failure.retries).toBe(3);
  });

  it("reads a missing last-try as null rather than the epoch", async () => {
    respondWith({ failures: [{ id: 1, to: "a@b.de" }] });

    const [failure] = await fetchEmailFailures();

    expect(failure.lastTry).toBeNull();
  });
});

describe("deleteEmailFailure", () => {
  it("deletes by id", async () => {
    respondWith({});

    await deleteEmailFailure(5);

    const [url, init] = fetchMock.mock.calls.at(-1) as [string, RequestInit];
    expect(url).toBe("/api/v2/admin/maintenance/email-failures/5");
    expect(init.method).toBe("DELETE");
  });
});
