import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

import { fetchConfig, resetConfig } from "./config";

/**
 * The deployment's own description, which cannot change while the page is open. What
 * matters is that asking twice costs one request: the footer mounts twice on the start
 * page, once per layout, and both ask.
 */

let fetchMock: ReturnType<typeof vi.fn>;

function respond(): Response {
  return new Response(
    JSON.stringify({
      branding: { title: "TUM-Live", description: "…" },
      versionTag: "development",
      wikiUrl: "",
      isFreshInstallation: false,
    }),
    { status: 200 },
  );
}

beforeEach(() => {
  resetConfig();
  fetchMock = vi.fn().mockImplementation(() => Promise.resolve(respond()));
  vi.stubGlobal("fetch", fetchMock);
});

afterEach(() => vi.unstubAllGlobals());

describe("fetchConfig", () => {
  it("shares one request between callers that ask at the same time", async () => {
    const [a, b] = await Promise.all([fetchConfig(), fetchConfig()]);

    expect(fetchMock).toHaveBeenCalledTimes(1);
    expect(a).toBe(b);
    expect(a.versionTag).toBe("development");
  });

  it("does not ask again once it has an answer", async () => {
    await fetchConfig();
    await fetchConfig();

    expect(fetchMock).toHaveBeenCalledTimes(1);
  });

  it("asks for no token, so it answers before anyone has signed in", async () => {
    await fetchConfig();

    expect(String(fetchMock.mock.calls[0][0])).toBe("/api/v2/config");
  });

  it("retries after a failure rather than caching it", async () => {
    fetchMock.mockResolvedValueOnce(new Response("nope", { status: 500 }));

    await expect(fetchConfig()).rejects.toThrow();
    await expect(fetchConfig()).resolves.toMatchObject({ versionTag: "development" });
    expect(fetchMock).toHaveBeenCalledTimes(2);
  });
});
