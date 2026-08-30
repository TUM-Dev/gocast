import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

import { clearToken } from "./api";
import { deleteRunner, fetchRunners, timeAgo } from "./runners";

/**
 * The runners list, and the two things this page derives from it that the old one
 * derived in an Alpine expression inside the template.
 */

let fetchMock: ReturnType<typeof vi.fn>;

interface WireRunner {
  hostname: string;
  port?: number;
  version?: string;
  alive?: boolean;
  jobCount?: string;
  draining?: boolean;
  /** protojson renders a Timestamp as an RFC 3339 string, not as {seconds}. */
  timeOfRegister?: string;
}

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

function withRunners(runners: WireRunner[]): void {
  respondWith({ runners });
}

beforeEach(() => {
  clearToken();
  fetchMock = vi.fn();
  vi.stubGlobal("fetch", fetchMock);
});

afterEach(() => {
  vi.unstubAllGlobals();
});

describe("fetchRunners", () => {
  it("reads a runner's fields off the wire", async () => {
    withRunners([
      {
        hostname: "runner-1",
        port: 50051,
        version: "1.2.3",
        alive: true,
        jobCount: "4",
        draining: true,
        timeOfRegister: "2023-11-14T22:13:20Z",
      },
    ]);

    const [runner] = await fetchRunners();

    expect(runner.hostname).toBe("runner-1");
    expect(runner.port).toBe(50051);
    expect(runner.version).toBe("1.2.3");
    expect(runner.alive).toBe(true);
    expect(runner.draining).toBe(true);
    expect(runner.registeredAt).toEqual(new Date("2023-11-14T22:13:20Z"));
  });

  it("reads the job count as a number", async () => {
    // protobuf uint64 arrives as a string, which renders as one and compares as one.
    // "10" < "9" lexically, so leaving it alone would sort and total incorrectly.
    withRunners([{ hostname: "runner-1", jobCount: "10" }]);

    const [runner] = await fetchRunners();

    expect(runner.jobCount).toBe(10);
  });

  it("sorts by hostname so rows do not move under the pointer", async () => {
    // The page refetches every five seconds and the endpoint returns database order.
    // Unsorted, a row could swap places between the click and the mouse-up on it.
    withRunners([{ hostname: "runner-c" }, { hostname: "runner-a" }, { hostname: "runner-b" }]);

    const runners = await fetchRunners();

    expect(runners.map((r) => r.hostname)).toEqual(["runner-a", "runner-b", "runner-c"]);
  });

  it("survives a runner that never registered", async () => {
    // time_of_register is absent from the JSON when it is the zero value, and the
    // page has to render the row rather than throwing on the date.
    withRunners([{ hostname: "runner-1" }]);

    const [runner] = await fetchRunners();

    expect(runner.registeredAt).toBeNull();
  });

  it("reads an empty list as no runners rather than as a failure", async () => {
    // protojson omits an empty repeated field, so the response body is `{}`.
    respondWith({});

    await expect(fetchRunners()).resolves.toEqual([]);
  });
});

describe("deleteRunner", () => {
  it("deletes by hostname", async () => {
    respondWith({});

    await deleteRunner("runner-1");

    const [url, init] = fetchMock.mock.calls.at(-1) as [string, RequestInit];
    expect(url).toBe("/api/v2/runners/runner-1");
    expect(init.method).toBe("DELETE");
  });

  it("escapes a hostname that would otherwise change the path", async () => {
    // Hostnames come from whatever registered itself, so they are not to be trusted
    // to be path-safe.
    respondWith({});

    await deleteRunner("odd/../name");

    const [url] = fetchMock.mock.calls.at(-1) as [string];
    expect(url).toBe("/api/v2/runners/odd%2F..%2Fname");
  });
});

describe("timeAgo", () => {
  const now = new Date("2026-08-30T12:00:00Z");

  it.each([
    ["just now", 0, "just now"],
    ["seconds", 5, "5 seconds"],
    ["one minute without a plural", 60, "1 minute"],
    ["minutes", 300, "5 minutes"],
    ["hours", 7200, "2 hours"],
    ["days", 172800, "2 days"],
  ])("renders %s", (_name, secondsAgo, expected) => {
    const from = new Date(now.getTime() - secondsAgo * 1000);

    expect(timeAgo(from, now)).toBe(expected);
  });

  it("says so when a runner has no registration time", () => {
    expect(timeAgo(null, now)).toBe("unknown");
  });
});
