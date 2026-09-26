import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

import { clearToken } from "./api";
import { deleteWorker, fetchWorkers } from "./workers";

let fetchMock: ReturnType<typeof vi.fn>;

interface WireWorker {
  workerId: string;
  host?: string;
  version?: string;
  alive?: boolean;
  workload?: number;
  status?: string;
  cpu?: string;
  memory?: string;
  disk?: string;
  uptime?: string;
  /** protojson renders a Timestamp as an RFC 3339 string, not as {seconds}. */
  lastSeen?: string;
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

function withWorkers(workers: WireWorker[], workerToken = ""): void {
  respondWith({ workers, workerToken });
}

beforeEach(() => {
  clearToken();
  fetchMock = vi.fn();
  vi.stubGlobal("fetch", fetchMock);
});

afterEach(() => {
  vi.unstubAllGlobals();
});

describe("fetchWorkers", () => {
  it("reads a worker's fields off the wire", async () => {
    withWorkers(
      [
        {
          workerId: "worker-1",
          host: "vm1234",
          version: "1.2.3",
          alive: true,
          workload: 4,
          status: "converting",
          cpu: "12%",
          memory: "1.2GB",
          disk: "20%",
          uptime: "3d",
          lastSeen: "2023-11-14T22:13:20Z",
        },
      ],
      "the-worker-token",
    );

    const { workers, token } = await fetchWorkers();
    const [worker] = workers;

    expect(worker.workerId).toBe("worker-1");
    expect(worker.host).toBe("vm1234");
    expect(worker.version).toBe("1.2.3");
    expect(worker.alive).toBe(true);
    expect(worker.workload).toBe(4);
    expect(worker.status).toBe("converting");
    expect(worker.cpu).toBe("12%");
    expect(worker.memory).toBe("1.2GB");
    expect(worker.disk).toBe("20%");
    expect(worker.uptime).toBe("3d");
    expect(worker.lastSeen).toEqual(new Date("2023-11-14T22:13:20Z"));
    expect(token).toBe("the-worker-token");
  });

  it("sorts by host so rows do not move under the pointer", async () => {
    // The page refetches every five seconds and the endpoint returns database order.
    // Unsorted, a row could swap places between the click and the mouse-up on it.
    withWorkers([
      { workerId: "3", host: "vm-c" },
      { workerId: "1", host: "vm-a" },
      { workerId: "2", host: "vm-b" },
    ]);

    const { workers } = await fetchWorkers();

    expect(workers.map((w) => w.host)).toEqual(["vm-a", "vm-b", "vm-c"]);
  });

  it("survives a worker that never sent a heartbeat", async () => {
    // last_seen is absent from the JSON when it is the zero value, and the page has
    // to render the row rather than throwing on the date.
    withWorkers([{ workerId: "worker-1", host: "vm1234" }]);

    const { workers } = await fetchWorkers();

    expect(workers[0].lastSeen).toBeNull();
  });

  it("reads an empty list as no workers rather than as a failure", async () => {
    // protojson omits an empty repeated field, so the response body is `{}`.
    respondWith({});

    await expect(fetchWorkers()).resolves.toEqual({ workers: [], token: "" });
  });
});

describe("deleteWorker", () => {
  it("deletes by worker id", async () => {
    respondWith({});

    await deleteWorker("worker-1");

    const [url, init] = fetchMock.mock.calls.at(-1) as [string, RequestInit];
    expect(url).toBe("/api/v2/admin/workers/worker-1");
    expect(init.method).toBe("DELETE");
  });

  it("escapes a worker id that would otherwise change the path", async () => {
    // Worker ids come from whatever registered itself, so they are not to be trusted
    // to be path-safe.
    respondWith({});

    await deleteWorker("odd/../id");

    const [url] = fetchMock.mock.calls.at(-1) as [string];
    expect(url).toBe("/api/v2/admin/workers/odd%2F..%2Fid");
  });
});
