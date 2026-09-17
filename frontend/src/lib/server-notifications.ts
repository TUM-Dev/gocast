/**
 * Server notifications: site-wide banners an administrator schedules between a start
 * and expiry time. MetaService.getServerNotifications (public) already serves the
 * active ones for the banner itself; this is the admin write side, which needs every
 * notification -- past, active and future -- plus its id, to list and edit them.
 */

import { timestampDate, timestampFromDate } from "@bufbuild/protobuf/wkt";

import {
  CreateServerNotificationRequestSchema,
  ListServerNotificationsAdminResponseSchema,
  ServerNotificationAdminSchema,
  UpdateServerNotificationRequestSchema,
  type ServerNotificationAdmin,
} from "@/gen/server/apiv2_pb";
import { apiDelete, apiGetMessage, apiPatchMessage, apiPostMessage } from "./api";

export interface AdminServerNotification {
  id: number;
  text: string;
  warn: boolean;
  start: Date;
  expires: Date;
}

function toAdminNotification(notification: ServerNotificationAdmin): AdminServerNotification {
  return {
    id: notification.id,
    text: notification.text,
    warn: notification.warn,
    start: notification.start ? timestampDate(notification.start) : new Date(0),
    expires: notification.expires ? timestampDate(notification.expires) : new Date(0),
  };
}

/** Every server notification, for the administration page. */
export async function fetchServerNotificationsAdmin(): Promise<AdminServerNotification[]> {
  const res = await apiGetMessage(
    ListServerNotificationsAdminResponseSchema,
    "/admin/server-notifications",
  );
  return res.notifications.map(toAdminNotification);
}

export async function createServerNotification(
  text: string,
  warn: boolean,
  start: Date,
  expires: Date,
): Promise<AdminServerNotification> {
  const created = await apiPostMessage(
    CreateServerNotificationRequestSchema,
    ServerNotificationAdminSchema,
    "/admin/server-notifications",
    {
      $typeName: "protobuf.CreateServerNotificationRequest",
      text,
      warn,
      start: timestampFromDate(start),
      expires: timestampFromDate(expires),
    },
  );
  return toAdminNotification(created);
}

export async function updateServerNotification(
  id: number,
  text: string,
  warn: boolean,
  start: Date,
  expires: Date,
): Promise<AdminServerNotification> {
  const updated = await apiPatchMessage(
    UpdateServerNotificationRequestSchema,
    ServerNotificationAdminSchema,
    `/admin/server-notifications/${id}`,
    {
      $typeName: "protobuf.UpdateServerNotificationRequest",
      id,
      text,
      warn,
      start: timestampFromDate(start),
      expires: timestampFromDate(expires),
    },
  );
  return toAdminNotification(updated);
}

export async function deleteServerNotification(id: number): Promise<void> {
  await apiDelete(`/admin/server-notifications/${id}`);
}
