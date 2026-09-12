import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

import { fetchInfoPage, isInfoPageName } from "./info-pages";

let fetchMock: ReturnType<typeof vi.fn>;

beforeEach(() => {
  fetchMock = vi.fn().mockResolvedValue(
    new Response(JSON.stringify({ name: "privacy", content: "<h1>Privacy</h1>" }), {
      status: 200,
    }),
  );
  vi.stubGlobal("fetch", fetchMock);
});

afterEach(() => vi.unstubAllGlobals());

describe("fetchInfoPage", () => {
  it("returns the server-rendered HTML unchanged", async () => {
    expect(await fetchInfoPage("privacy")).toBe("<h1>Privacy</h1>");
  });

  it("asks for the page by route name", async () => {
    await fetchInfoPage("imprint");

    expect(fetchMock.mock.calls[0][0]).toBe("/api/v2/info-pages/imprint");
  });

  it("sends no Authorization header", async () => {
    // A token minted from a session that does not exist fails for the wrong reason.
    await fetchInfoPage("about");

    const init = fetchMock.mock.calls[0][1] as RequestInit;
    expect(init.headers).not.toHaveProperty("Authorization");
    expect(fetchMock).toHaveBeenCalledTimes(1);
  });

  it("propagates a failure rather than returning empty content", async () => {
    fetchMock.mockResolvedValue(new Response("{}", { status: 404 }));

    await expect(fetchInfoPage("privacy")).rejects.toThrow();
  });
});

describe("isInfoPageName", () => {
  it("accepts the three pages the server serves", () => {
    expect(isInfoPageName("privacy")).toBe(true);
    expect(isInfoPageName("imprint")).toBe(true);
    expect(isInfoPageName("about")).toBe(true);
  });

  it("rejects anything else", () => {
    expect(isInfoPageName("terms")).toBe(false);
  });
});
