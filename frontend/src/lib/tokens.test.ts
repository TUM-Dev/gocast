import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

import { clearToken } from "./api";
import { createToken, deleteToken, fetchTokens, tokenOwner, type AdminToken } from "./tokens";

/**
 * The token list, and the two client-only helpers that used to be inline Alpine
 * expressions in the old template: which name to show for a row, and turning an
 * expiry date into the wire's Timestamp shape.
 */

let fetchMock: ReturnType<typeof vi.fn>;

interface WireToken {
  id?: number;
  userName?: string;
  userEmail?: string;
  userLrzId?: string;
  scope?: string;
  /** protojson renders a Timestamp as an RFC 3339 string, not as {seconds}. */
  expires?: string;
  lastUse?: string;
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

function withTokens(tokens: WireToken[], rtmpProxyUrl = ""): void {
  respondWith({ tokens, rtmpProxyUrl });
}

beforeEach(() => {
  clearToken();
  fetchMock = vi.fn();
  vi.stubGlobal("fetch", fetchMock);
});

afterEach(() => {
  vi.unstubAllGlobals();
});

describe("fetchTokens", () => {
  it("reads a token's fields off the wire, without a secret", async () => {
    withTokens([
      {
        id: 7,
        userName: "Anja Admin",
        userEmail: "anja@example.org",
        scope: "lecturer",
        expires: "2030-01-01T00:00:00Z",
        lastUse: "2026-01-01T00:00:00Z",
      },
    ]);

    const { tokens } = await fetchTokens();
    const [token] = tokens;

    expect(token.id).toBe(7);
    expect(token.userName).toBe("Anja Admin");
    expect(token.scope).toBe("lecturer");
    expect(token.expires).toEqual(new Date("2030-01-01T00:00:00Z"));
    expect(token.lastUse).toEqual(new Date("2026-01-01T00:00:00Z"));
    // The wire type has no field for the secret at all, so there is nothing to assert
    // beyond it not appearing here -- this test would not compile otherwise.
  });

  it("reads a token that has never expired or been used", async () => {
    withTokens([{ id: 1, userEmail: "a@b.c", scope: "admin" }]);

    const { tokens } = await fetchTokens();
    const [token] = tokens;

    expect(token.expires).toBeNull();
    expect(token.lastUse).toBeNull();
  });

  it("reads an empty list as no tokens rather than as a failure", async () => {
    respondWith({});

    await expect(fetchTokens()).resolves.toEqual({ tokens: [], rtmpProxyUrl: "" });
  });

  it("reads the rtmp proxy url alongside the tokens", async () => {
    withTokens([], "rtmp://ingest.example.org/live");

    const { rtmpProxyUrl } = await fetchTokens();

    expect(rtmpProxyUrl).toBe("rtmp://ingest.example.org/live");
  });
});

describe("tokenOwner", () => {
  const base: AdminToken = {
    id: 1,
    userName: "Peter Prof",
    userEmail: "",
    userLrzId: "",
    scope: "lecturer",
    expires: null,
    lastUse: null,
  };

  it("prefers the email when there is one", () => {
    expect(tokenOwner({ ...base, userEmail: "peter@example.org" })).toBe("peter@example.org");
  });

  it("falls back to the name and LRZ id", () => {
    expect(tokenOwner({ ...base, userLrzId: "ab12cde" })).toBe("Peter Prof ab12cde");
  });
});

describe("createToken", () => {
  it("posts the scope and returns the secret", async () => {
    respondWith({ token: "generated-secret" });

    const secret = await createToken("lecturer", null);

    expect(secret).toBe("generated-secret");
    const [url, init] = fetchMock.mock.calls.at(-1) as [string, RequestInit];
    expect(url).toBe("/api/v2/admin/tokens");
    expect(init.method).toBe("POST");
    expect(JSON.parse(init.body as string)).toEqual({ scope: "lecturer" });
  });

  it("sends an expiry date as a Timestamp", async () => {
    respondWith({ token: "generated-secret" });

    await createToken("admin", new Date("2030-06-15T00:00:00Z"));

    const [, init] = fetchMock.mock.calls.at(-1) as [string, RequestInit];
    const body = JSON.parse(init.body as string);
    expect(body.scope).toBe("admin");
    expect(body.expires).toBe("2030-06-15T00:00:00Z");
  });
});

describe("deleteToken", () => {
  it("deletes by id", async () => {
    respondWith({});

    await deleteToken(7);

    const [url, init] = fetchMock.mock.calls.at(-1) as [string, RequestInit];
    expect(url).toBe("/api/v2/admin/tokens/7");
    expect(init.method).toBe("DELETE");
  });
});
