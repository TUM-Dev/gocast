import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

import { fetchInfoPage } from "./info-pages";

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

  it("asks for the page by slug", async () => {
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

describe("isInfoPageSlug", () => {
  // The list is cached at module scope, so each case needs its own fresh module
  // instance rather than sharing one poisoned by a previous case's mock.
  async function freshIsInfoPageSlug() {
    vi.resetModules();
    return (await import("./info-pages")).isInfoPageSlug;
  }

  it("accepts a slug the list endpoint names", async () => {
    fetchMock.mockResolvedValue(
      new Response(JSON.stringify({ pages: [{ slug: "privacy", name: "Privacy Policy" }] }), {
        status: 200,
      }),
    );

    expect(await (await freshIsInfoPageSlug())("privacy")).toBe(true);
  });

  it("rejects a slug the list endpoint does not name", async () => {
    fetchMock.mockResolvedValue(
      new Response(JSON.stringify({ pages: [{ slug: "privacy", name: "Privacy Policy" }] }), {
        status: 200,
      }),
    );

    expect(await (await freshIsInfoPageSlug())("terms")).toBe(false);
  });

  it("asks the list endpoint once no matter how many slugs are checked", async () => {
    fetchMock.mockResolvedValue(
      new Response(JSON.stringify({ pages: [{ slug: "privacy", name: "Privacy Policy" }] }), {
        status: 200,
      }),
    );
    const check = await freshIsInfoPageSlug();

    await check("privacy");
    await check("imprint");

    expect(fetchMock).toHaveBeenCalledTimes(1);
  });

  it("reports false, not a thrown error, when the list endpoint fails", async () => {
    fetchMock.mockResolvedValue(new Response("{}", { status: 500 }));

    expect(await (await freshIsInfoPageSlug())("privacy")).toBe(false);
  });
});
