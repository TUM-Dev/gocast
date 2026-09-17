import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

import { clearToken } from "./api";
import {
  createNotification,
  deleteNotification,
  fetchNotificationsAdmin,
  NotificationTarget,
} from "./notifications-admin";

let fetchMock: ReturnType<typeof vi.fn>;

interface WireNotification {
  id?: number;
  title?: string;
  body?: string;
  target?: number;
  /** protojson renders a Timestamp as an RFC 3339 string, not as {seconds}. */
  createdAt?: string;
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

function withNotifications(notifications: WireNotification[]): void {
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

describe("fetchNotificationsAdmin", () => {
  it("reads a notification's fields off the wire", async () => {
    withNotifications([
      {
        id: 7,
        title: "Maintenance",
        body: "<p>planned downtime</p>",
        target: NotificationTarget.Admin,
        createdAt: "2023-11-14T22:13:20Z",
      },
    ]);

    const [notification] = await fetchNotificationsAdmin();

    expect(notification.id).toBe(7);
    expect(notification.title).toBe("Maintenance");
    expect(notification.body).toBe("<p>planned downtime</p>");
    expect(notification.target).toBe(NotificationTarget.Admin);
    expect(notification.createdAt).toEqual(new Date("2023-11-14T22:13:20Z"));
  });

  it("reports a missing title as undefined, not an empty string", async () => {
    withNotifications([{ id: 1, body: "no title here", target: NotificationTarget.All }]);

    const [notification] = await fetchNotificationsAdmin();

    expect(notification.title).toBeUndefined();
  });

  it("reads an empty list as no notifications rather than as a failure", async () => {
    // protojson omits an empty repeated field, so the response body is `{}`.
    respondWith({});

    await expect(fetchNotificationsAdmin()).resolves.toEqual([]);
  });
});

describe("createNotification", () => {
  it("posts the title, body and target", async () => {
    respondWith({ id: 3, body: "<p>hi</p>", target: NotificationTarget.Student });

    await createNotification("hi", NotificationTarget.Student, "Hello");

    const [url, init] = fetchMock.mock.calls.at(-1) as [string, RequestInit];
    expect(url).toBe("/api/v2/admin/notifications");
    expect(init.method).toBe("POST");
    expect(JSON.parse(init.body as string)).toEqual({
      title: "Hello",
      body: "hi",
      target: NotificationTarget.Student,
    });
  });

  it("defaults to no title", async () => {
    respondWith({ id: 3, body: "<p>hi</p>", target: NotificationTarget.All });

    await createNotification("hi", NotificationTarget.All);

    const [, init] = fetchMock.mock.calls.at(-1) as [string, RequestInit];
    expect(JSON.parse(init.body as string)).not.toHaveProperty("title");
  });

  it("returns the created notification, rendered", async () => {
    respondWith({ id: 3, title: "Hello", body: "<p>hi</p>", target: NotificationTarget.All });

    const created = await createNotification("hi", NotificationTarget.All, "Hello");

    expect(created).toEqual({ id: 3, title: "Hello", body: "<p>hi</p>", target: NotificationTarget.All, createdAt: null });
  });
});

describe("deleteNotification", () => {
  it("deletes by id", async () => {
    respondWith({});

    await deleteNotification(9);

    const [url, init] = fetchMock.mock.calls.at(-1) as [string, RequestInit];
    expect(url).toBe("/api/v2/admin/notifications/9");
    expect(init.method).toBe("DELETE");
  });
});
