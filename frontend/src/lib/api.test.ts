import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

import { ApiError, apiFetchOptionalAuth, apiGet, clearToken } from "./api";

/**
 * These cover the credential handling rather than any single endpoint: the token is
 * minted lazily, shared between concurrent callers, and refreshed once when the server
 * rejects it. Only `fetch` is stubbed, so the real client code runs.
 */

const TOKEN_URL = "/api/v2/auth/token";

function tokenResponse(token = "token-1"): Response {
  return new Response(JSON.stringify({ access_token: token, expires_in: 900 }), { status: 200 });
}

function jsonResponse(body: unknown, status = 200): Response {
  return new Response(JSON.stringify(body), { status });
}

let fetchMock: ReturnType<typeof vi.fn>;

function calls(): string[] {
  return fetchMock.mock.calls.map((call) => String(call[0]));
}

function authHeaderOf(callIndex: number): string | null {
  const init = fetchMock.mock.calls[callIndex][1] as RequestInit | undefined;
  const headers = (init?.headers ?? {}) as Record<string, string>;
  return headers.Authorization ?? null;
}

beforeEach(() => {
  clearToken();
  fetchMock = vi.fn();
  vi.stubGlobal("fetch", fetchMock);
});

afterEach(() => {
  vi.unstubAllGlobals();
});

describe("apiGet", () => {
  it("mints a token and sends it as a bearer credential", async () => {
    fetchMock.mockResolvedValueOnce(tokenResponse()).mockResolvedValueOnce(jsonResponse({ ok: 1 }));

    await apiGet("/users/me");

    expect(calls()).toEqual([TOKEN_URL, "/api/v2/users/me"]);
    expect(authHeaderOf(1)).toBe("Bearer token-1");
  });

  it("reuses the token across requests", async () => {
    // A Response body can only be read once, so each call needs a fresh one.
    fetchMock
      .mockResolvedValueOnce(tokenResponse())
      .mockImplementation(() => Promise.resolve(jsonResponse({ ok: 1 })));

    await apiGet("/users/me");
    await apiGet("/semesters");

    expect(calls().filter((url) => url === TOKEN_URL)).toHaveLength(1);
  });

  it("mints only one token for concurrent requests", async () => {
    // Without single-flight, a page issuing several requests at once would mint a
    // separate token for each of them.
    fetchMock.mockImplementation((url: string) =>
      Promise.resolve(url === TOKEN_URL ? tokenResponse() : jsonResponse({ ok: 1 })),
    );

    await Promise.all([apiGet("/users/me"), apiGet("/semesters"), apiGet("/notifications")]);

    expect(calls().filter((url) => url === TOKEN_URL)).toHaveLength(1);
  });

  it("refreshes once and retries when the token is rejected", async () => {
    fetchMock
      .mockResolvedValueOnce(tokenResponse("stale"))
      .mockResolvedValueOnce(jsonResponse({ message: "expired" }, 401))
      .mockResolvedValueOnce(tokenResponse("fresh"))
      .mockResolvedValueOnce(jsonResponse({ ok: 1 }));

    await expect(apiGet("/users/me")).resolves.toEqual({ ok: 1 });

    expect(calls()).toEqual([TOKEN_URL, "/api/v2/users/me", TOKEN_URL, "/api/v2/users/me"]);
    expect(authHeaderOf(1)).toBe("Bearer stale");
    expect(authHeaderOf(3)).toBe("Bearer fresh");
  });

  it("gives up when the session itself is gone", async () => {
    fetchMock
      .mockResolvedValueOnce(tokenResponse())
      .mockResolvedValueOnce(jsonResponse({ message: "expired" }, 401))
      .mockResolvedValueOnce(jsonResponse({ error: "no active session" }, 401));

    const err = await apiGet("/users/me").catch((e: unknown) => e);

    expect(err).toBeInstanceOf(ApiError);
    expect((err as ApiError).isUnauthenticated).toBe(true);
  });

  it("surfaces the gateway's error message", async () => {
    fetchMock
      .mockResolvedValueOnce(tokenResponse())
      .mockResolvedValueOnce(jsonResponse({ code: 7, message: "not allowed" }, 403));

    const err = await apiGet("/users/me").catch((e: unknown) => e);

    expect(err).toBeInstanceOf(ApiError);
    expect((err as ApiError).status).toBe(403);
    expect((err as ApiError).message).toBe("not allowed");
  });

  it("does not retry a non-authentication failure", async () => {
    fetchMock
      .mockResolvedValueOnce(tokenResponse())
      .mockResolvedValueOnce(jsonResponse({ message: "boom" }, 500));

    await expect(apiGet("/users/me")).rejects.toBeInstanceOf(ApiError);
    expect(calls()).toHaveLength(2);
  });
});

/**
 * The listings that answer anonymous callers but show a signed-in one more. Getting
 * this wrong is silent both ways: an anonymous visitor sees an error page for being
 * logged out, or a signed-in user quietly sees the logged-out view of the site.
 */
describe("apiFetchOptionalAuth", () => {
  it("sends the token when there is a session", async () => {
    fetchMock.mockResolvedValueOnce(tokenResponse()).mockResolvedValueOnce(jsonResponse({ ok: 1 }));

    await apiFetchOptionalAuth("/courses");

    expect(calls()).toEqual([TOKEN_URL, "/api/v2/courses"]);
    expect(authHeaderOf(1)).toBe("Bearer token-1");
  });

  it("falls back to an unauthenticated request when nobody is signed in", async () => {
    fetchMock
      .mockResolvedValueOnce(jsonResponse({ message: "no session" }, 401))
      .mockResolvedValueOnce(jsonResponse({ ok: 1 }));

    await apiFetchOptionalAuth("/courses");

    expect(calls()).toEqual([TOKEN_URL, "/api/v2/courses"]);
    expect(authHeaderOf(1)).toBeNull();
  });

  it("does not ask for a token again once there is known to be no session", async () => {
    fetchMock
      .mockResolvedValueOnce(jsonResponse({ message: "no session" }, 401))
      .mockImplementation(() => Promise.resolve(jsonResponse({ ok: 1 })));

    await apiFetchOptionalAuth("/courses");
    await apiFetchOptionalAuth("/courses/live");

    // One doomed token request for the page, not one per listing.
    expect(calls()).toEqual([TOKEN_URL, "/api/v2/courses", "/api/v2/courses/live"]);
  });

  it("propagates a token failure that is not a missing session", async () => {
    // A 500 leaves the question open; treating it as "logged out" would show the
    // anonymous view to a user who is signed in.
    fetchMock.mockResolvedValueOnce(jsonResponse({ message: "boom" }, 500));

    await expect(apiFetchOptionalAuth("/courses")).rejects.toBeInstanceOf(ApiError);
    expect(calls()).toEqual([TOKEN_URL]);
  });
});
