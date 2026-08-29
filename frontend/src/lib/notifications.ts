/**
 * User notifications for the header bell.
 *
 * Read state is client-side only, so the list is cached in localStorage and refetched
 * at most every ten minutes.
 *
 * The keys are deliberately not the ones web/ts/notifications.ts uses. Sharing them
 * looked like it would carry read state between the two frontends, but the entries are
 * not interchangeable: the legacy list matches on a database `id`, which
 * protobuf.UserGroupNotification has no field for, and this one matches on a `key`
 * derived from the content, which legacy entries lack. Whichever wrote last, the other
 * matched nothing — showing every notification unread, or rendering rows keyed
 * `undefined`. Adding `id` to the proto would make one shared cache possible.
 */

import {
  GetNotificationsResponseSchema,
  GetServerNotificationsResponseSchema,
} from "@/gen/server/apiv2_pb";
import { apiGetMessage, apiGetMessagePublic } from "./api";

const STORAGE_KEY = "spa.notifications";
const LAST_FETCH_KEY = "spa.lastNotificationFetch";
const REFETCH_AFTER_MS = 10 * 60 * 1000;

export interface Notification {
  /**
   * Stable identity for read tracking, derived from the content because
   * protobuf.UserGroupNotification has no id field. Adding one would make it exact.
   */
  key: string;
  title?: string;
  body: string;
  read: boolean;
}

function readCache(): Notification[] {
  try {
    const cached = JSON.parse(localStorage.getItem(STORAGE_KEY) ?? "[]");
    return Array.isArray(cached) ? (cached as Notification[]) : [];
  } catch {
    return [];
  }
}

function writeCache(notifications: Notification[]): void {
  localStorage.setItem(STORAGE_KEY, JSON.stringify(notifications));
}

/** Marks everything currently known as read, so the bell's indicator clears. */
export function markAllRead(notifications: Notification[]): void {
  for (const notification of notifications) {
    notification.read = true;
  }
  writeCache(notifications);
}

/**
 * Returns the user's notifications, preserving read flags for ones already seen, and
 * serving the cache without a request when the last fetch was recent.
 */
export async function fetchNotifications(force = false): Promise<Notification[]> {
  const cached = readCache();
  const lastFetch = Number.parseInt(localStorage.getItem(LAST_FETCH_KEY) ?? "0", 10);

  if (!force && Date.now() - lastFetch <= REFETCH_AFTER_MS) {
    return cached;
  }

  const readByKey = new Map(cached.map((n) => [n.key, n.read]));
  const res = await apiGetMessage(GetNotificationsResponseSchema, "/notifications");

  const notifications: Notification[] = res.notifications.map((n) => {
    const key = `${n.createdAt?.seconds ?? 0}:${n.title}:${n.body}`;
    return {
      key,
      title: n.title === "" ? undefined : n.title,
      body: n.body,
      read: readByKey.get(key) ?? false,
    };
  });

  writeCache(notifications);
  localStorage.setItem(LAST_FETCH_KEY, Date.now().toString());

  return notifications;
}

/**
 * A banner the operators put across the top of the start page — planned downtime, a
 * degraded service. Unrelated to the notifications above: nobody is the recipient, so
 * there is no read state and nothing is cached.
 */
export interface ServerNotification {
  /** Raw HTML, written by an administrator and rendered as markup on the old page. */
  html: string;
  /** Warnings are styled as such; the rest are informational. */
  warn: boolean;
}

/** Current server notifications. Answers anonymous callers, as the banner is for all. */
export async function fetchServerNotifications(): Promise<ServerNotification[]> {
  const res = await apiGetMessagePublic(
    GetServerNotificationsResponseSchema,
    "/server-notifications",
  );

  return res.serverNotifications.map((n) => ({ html: n.text, warn: n.warn }));
}
