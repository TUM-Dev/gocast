import { afterEach, beforeEach, expect, it, vi } from "vitest";

import { clearToken } from "./api";
import {
  createIntegration,
  fetchIntegrations,
  revokeIntegrationKey,
  rotateIntegrationKey,
} from "./integrations";

let fetchMock: ReturnType<typeof vi.fn>;

function respondWith(body: unknown, status = 200): void {
  fetchMock.mockImplementation((url: string) =>
    Promise.resolve(
      new Response(JSON.stringify(url.endsWith("/auth/token") ? { access_token: "t" } : body), {
        status: url.endsWith("/auth/token") ? 200 : status,
      }),
    ),
  );
}

beforeEach(() => {
  clearToken();
  fetchMock = vi.fn();
  vi.stubGlobal("fetch", fetchMock);
});
afterEach(() => vi.unstubAllGlobals());

it("reads integration details and defaults an absent key status to revoked", async () => {
  respondWith({
    integrations: [
      {
        id: 7,
        name: "Portal",
        returnUrl: "https://portal.example/callback",
        hasKey: true,
      },
      { id: 8 },
    ],
  });
  const rows = await fetchIntegrations();
  expect(rows[0]).toMatchObject({
    id: 7,
    name: "Portal",
    returnUrl: "https://portal.example/callback",
    hasKey: true,
  });
  expect(rows[1].hasKey).toBe(false);
  expect(fetchMock.mock.calls.at(-1)?.[0]).toBe("/api/v2/admin/integrations");
});

it("accepts an empty list and propagates errors", async () => {
  respondWith({});
  expect(await fetchIntegrations()).toEqual([]);
  respondWith({ message: "Forbidden" }, 403);
  await expect(fetchIntegrations()).rejects.toThrow();
});

it("registers an application and returns the one-time key", async () => {
  respondWith({
    id: 7,
    name: "Portal",
    returnUrl: "https://portal.example/callback",
    apiKey: "new-key",
  });
  expect((await createIntegration("Portal", "https://portal.example/callback")).apiKey).toBe(
    "new-key",
  );
  const [url, init] = fetchMock.mock.calls.at(-1) as [string, RequestInit];
  expect(url).toBe("/api/v2/admin/integrations");
  expect(init.method).toBe("POST");
  expect(JSON.parse(init.body as string)).toEqual({
    name: "Portal",
    returnUrl: "https://portal.example/callback",
  });
});

it("rotates and revokes the selected application's key", async () => {
  respondWith({ apiKey: "replacement" });
  expect(await rotateIntegrationKey(7)).toBe("replacement");
  expect(fetchMock.mock.calls.at(-1)?.[0]).toBe("/api/v2/admin/integrations/7/key");
  expect(fetchMock.mock.calls.at(-1)?.[1].method).toBe("POST");
  respondWith({});
  await revokeIntegrationKey(7);
  expect(fetchMock.mock.calls.at(-1)?.[0]).toBe("/api/v2/admin/integrations/7/key");
  expect(fetchMock.mock.calls.at(-1)?.[1].method).toBe("DELETE");
});
