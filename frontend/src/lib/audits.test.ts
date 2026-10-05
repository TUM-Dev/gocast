import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

import { clearToken } from "./api";
import { AUDITS_PAGE_SIZE, fetchAudits } from "./audits";

let fetchMock: ReturnType<typeof vi.fn>;

interface WireAuditEntry {
  id?: number;
  createdAt?: string;
  type?: string;
  message?: string;
  userId?: number;
  userName?: string;
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

function withAudits(audits: WireAuditEntry[]): void {
  respondWith({ audits });
}

beforeEach(() => {
  clearToken();
  fetchMock = vi.fn();
  vi.stubGlobal("fetch", fetchMock);
});

afterEach(() => {
  vi.unstubAllGlobals();
});

describe("fetchAudits", () => {
  it("reads an entry's fields off the wire", async () => {
    withAudits([
      {
        id: 7,
        createdAt: "2023-11-14T22:13:20Z",
        type: "Camera Moved",
        message: "Camera moved to preset 2",
        userId: 1,
        userName: "admin@example.com",
      },
    ]);

    const [entry] = await fetchAudits(0);

    expect(entry.id).toBe(7);
    expect(entry.createdAt).toEqual(new Date("2023-11-14T22:13:20Z"));
    expect(entry.type).toBe("Camera Moved");
    expect(entry.message).toBe("Camera moved to preset 2");
    expect(entry.userId).toBe(1);
    expect(entry.userName).toBe("admin@example.com");
  });

  it("survives a system audit with no user", async () => {
    // protojson omits a zero uint32, so userId is simply absent from the JSON.
    withAudits([{ id: 1, type: "Info", message: "server started", userName: "- System -" }]);

    const [entry] = await fetchAudits(0);

    expect(entry.userId).toBe(0);
    expect(entry.userName).toBe("- System -");
  });

  it("reads an empty page as no entries rather than as a failure", async () => {
    // protojson omits an empty repeated field, so the response body is `{}`.
    respondWith({});

    await expect(fetchAudits(0)).resolves.toEqual([]);
  });

  it("asks for the requested page", async () => {
    withAudits([]);

    await fetchAudits(20, 5);

    const [url] = fetchMock.mock.calls.at(-1) as [string];
    expect(url).toBe(`/api/v2/admin/audits?limit=5&offset=20`);
  });

  it("defaults the limit to the old page's page size", async () => {
    withAudits([]);

    await fetchAudits(0);

    const [url] = fetchMock.mock.calls.at(-1) as [string];
    expect(url).toBe(`/api/v2/admin/audits?limit=${AUDITS_PAGE_SIZE}&offset=0`);
  });
});
