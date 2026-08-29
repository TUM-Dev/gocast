import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

import { clearToken } from "./api";
import { fetchNotifications, markAllRead } from "./notifications";

/**
 * Read state is a client-side concept the API knows nothing about, so it has to
 * survive refetches. It is keyed on the notification's content because the proto
 * carries no id — these tests describe what that buys and what it costs.
 */

let fetchMock: ReturnType<typeof vi.fn>;

function respondWith(notifications: { title: string; body: string; createdAt?: string }[]): void {
  fetchMock.mockImplementation((url: string) => {
    if (String(url).endsWith("/auth/token")) {
      return Promise.resolve(
        new Response(JSON.stringify({ access_token: "t", expires_in: 900 }), { status: 200 }),
      );
    }
    return Promise.resolve(new Response(JSON.stringify({ notifications }), { status: 200 }));
  });
}

beforeEach(() => {
  clearToken();
  localStorage.clear();
  fetchMock = vi.fn();
  vi.stubGlobal("fetch", fetchMock);
});

afterEach(() => {
  vi.unstubAllGlobals();
});

describe("fetchNotifications", () => {
  it("returns notifications as unread the first time they are seen", async () => {
    respondWith([{ title: "Maintenance", body: "Tonight" }]);

    const notifications = await fetchNotifications();

    expect(notifications).toHaveLength(1);
    expect(notifications[0].read).toBe(false);
  });

  it("keeps notifications read across a refetch", async () => {
    respondWith([{ title: "Maintenance", body: "Tonight" }]);

    const first = await fetchNotifications();
    markAllRead(first);
    const second = await fetchNotifications(true);

    expect(second[0].read).toBe(true);
  });

  it("treats an edited notification as new", async () => {
    // The cost of keying on content rather than an id: changing the text server-side
    // makes a notification unread again. An id field in the proto would fix this.
    respondWith([{ title: "Maintenance", body: "Tonight" }]);
    markAllRead(await fetchNotifications());

    respondWith([{ title: "Maintenance", body: "Tomorrow" }]);
    const updated = await fetchNotifications(true);

    expect(updated[0].read).toBe(false);
  });

  it("serves the cache instead of refetching straight away", async () => {
    respondWith([{ title: "Maintenance", body: "Tonight" }]);
    await fetchNotifications();
    const callsAfterFirst = fetchMock.mock.calls.length;

    const cached = await fetchNotifications();

    expect(cached).toHaveLength(1);
    expect(fetchMock.mock.calls.length).toBe(callsAfterFirst);
  });

  it("survives a corrupted cache", async () => {
    localStorage.setItem("notifications", "not json");
    respondWith([{ title: "Maintenance", body: "Tonight" }]);

    await expect(fetchNotifications(true)).resolves.toHaveLength(1);
  });
});
