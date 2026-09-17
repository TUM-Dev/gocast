import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

import { clearToken } from "./api";
import {
  createServerNotification,
  deleteServerNotification,
  fetchServerNotificationsAdmin,
  updateServerNotification,
} from "./server-notifications";

let fetchMock: ReturnType<typeof vi.fn>;

interface WireServerNotification {
  id?: number;
  text?: string;
  warn?: boolean;
  /** protojson renders a Timestamp as an RFC 3339 string, not as {seconds}. */
  start?: string;
  expires?: string;
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

function withNotifications(notifications: WireServerNotification[]): void {
  respondWith({ notifications });
}

beforeEach(() => {
  clearToken();
  fetchMock = vi.fn();
  vi.stubGlobal("fetch", fetchMock);
});

afterEach(() => {
  vi.unstubAllGlobals();
});

describe("fetchServerNotificationsAdmin", () => {
  it("reads a notification's fields off the wire", async () => {
    withNotifications([
      {
        id: 3,
        text: "Maintenance this weekend.",
        warn: true,
        start: "2023-11-14T22:13:20Z",
        expires: "2023-11-21T22:13:20Z",
      },
    ]);

    const [notification] = await fetchServerNotificationsAdmin();

    expect(notification.id).toBe(3);
    expect(notification.text).toBe("Maintenance this weekend.");
    expect(notification.warn).toBe(true);
    expect(notification.start).toEqual(new Date("2023-11-14T22:13:20Z"));
    expect(notification.expires).toEqual(new Date("2023-11-21T22:13:20Z"));
  });

  it("reads an empty list as no notifications rather than as a failure", async () => {
    // protojson omits an empty repeated field, so the response body is `{}`.
    respondWith({});

    await expect(fetchServerNotificationsAdmin()).resolves.toEqual([]);
  });
});

describe("createServerNotification", () => {
  it("posts the message, severity and window, and reads back the created row", async () => {
    respondWith({
      id: 9,
      text: "New notification",
      warn: false,
      start: "2026-01-01T00:00:00Z",
      expires: "2026-01-02T00:00:00Z",
    });

    const created = await createServerNotification(
      "New notification",
      false,
      new Date("2026-01-01T00:00:00Z"),
      new Date("2026-01-02T00:00:00Z"),
    );

    expect(created.id).toBe(9);
    const [url, init] = fetchMock.mock.calls.at(-1) as [string, RequestInit];
    expect(url).toBe("/api/v2/admin/server-notifications");
    expect(init.method).toBe("POST");
    const body = JSON.parse(init.body as string);
    expect(body.text).toBe("New notification");
    expect(body.start).toBe("2026-01-01T00:00:00Z");
  });
});

describe("updateServerNotification", () => {
  it("patches the given id", async () => {
    respondWith({
      id: 5,
      text: "Updated",
      warn: true,
      start: "2026-01-01T00:00:00Z",
      expires: "2026-01-02T00:00:00Z",
    });

    await updateServerNotification(
      5,
      "Updated",
      true,
      new Date("2026-01-01T00:00:00Z"),
      new Date("2026-01-02T00:00:00Z"),
    );

    const [url, init] = fetchMock.mock.calls.at(-1) as [string, RequestInit];
    expect(url).toBe("/api/v2/admin/server-notifications/5");
    expect(init.method).toBe("PATCH");
  });
});

describe("deleteServerNotification", () => {
  it("deletes by id", async () => {
    respondWith({});

    await deleteServerNotification(5);

    const [url, init] = fetchMock.mock.calls.at(-1) as [string, RequestInit];
    expect(url).toBe("/api/v2/admin/server-notifications/5");
    expect(init.method).toBe("DELETE");
  });
});
