/**
 * Administration of user notifications: the banner messages broadcast to a group of
 * users, not the per-viewer read state in lib/notifications.ts (a different model
 * entirely -- see that file's header for why the two do not share a cache).
 */

import { timestampDate } from "@bufbuild/protobuf/wkt";

import {
  AdminNotificationSchema,
  CreateNotificationRequestSchema,
  ListNotificationsAdminResponseSchema,
  type AdminNotification,
} from "@/gen/server/apiv2_pb";
import { apiDelete, apiGetMessage, apiPostMessage } from "./api";

/**
 * Who a notification is shown to. Numbered from 1 to match model.NotificationTarget
 * in the Go backend -- see AdminNotification's `target` field for why this is a plain
 * number rather than the sibling NotificationTarget enum.
 */
export enum NotificationTarget {
  All = 1,
  LoggedIn = 2,
  Student = 3,
  Lecturer = 4,
  Admin = 5,
}

export const notificationTargetLabels: Record<NotificationTarget, string> = {
  [NotificationTarget.All]: "All Users",
  [NotificationTarget.LoggedIn]: "Logged-in Users",
  [NotificationTarget.Student]: "Students",
  [NotificationTarget.Lecturer]: "Lecturers",
  [NotificationTarget.Admin]: "Admins",
};

export interface Notification {
  id: number;
  /** Undefined when the notification has no title, rather than an empty string. */
  title?: string;
  /** Rendered from Markdown and sanitised server-side; the client must not do it again. */
  body: string;
  target: NotificationTarget;
  createdAt: Date | null;
}

function toNotification(n: AdminNotification): Notification {
  return {
    id: n.id,
    title: n.title === "" ? undefined : n.title,
    body: n.body,
    target: n.target as NotificationTarget,
    createdAt: n.createdAt ? timestampDate(n.createdAt) : null,
  };
}

/** Every notification ever broadcast, newest first, for the administration page. */
export async function fetchNotificationsAdmin(): Promise<Notification[]> {
  const res = await apiGetMessage(ListNotificationsAdminResponseSchema, "/admin/notifications");
  return res.notifications.map(toNotification);
}

/** Broadcasts a notification to users matching its target group. */
export async function createNotification(
  body: string,
  target: NotificationTarget,
  title = "",
): Promise<Notification> {
  const created = await apiPostMessage(
    CreateNotificationRequestSchema,
    AdminNotificationSchema,
    "/admin/notifications",
    { $typeName: "protobuf.CreateNotificationRequest", title, body, target },
  );
  return toNotification(created);
}

/** Deletes a notification; it stops showing to users immediately. */
export async function deleteNotification(id: number): Promise<void> {
  await apiDelete(`/admin/notifications/${id}`);
}
