import { create } from "@bufbuild/protobuf";
import { timestampDate } from "@bufbuild/protobuf/wkt";

import {
  OfflineLectureSchema,
  RequestOfflineLectureRequestSchema,
  SyncOfflineLicensesRequestSchema,
  SyncOfflineLicensesResponseSchema,
  type OfflineLecture,
} from "@/gen/server/apiv2_pb";
import { apiDelete, apiGetMessage, apiPost, apiPostMessage } from "@/lib/api";

/** The server's side of saving lectures offline (apiv2/server/offline.go). */

export function requestOfflineLecture(streamId: number): Promise<OfflineLecture> {
  return apiPostMessage(
    RequestOfflineLectureRequestSchema,
    OfflineLectureSchema,
    `/streams/${streamId}/offline`,
    create(RequestOfflineLectureRequestSchema, { streamId }),
  );
}

export function getOfflineLecture(packageId: number): Promise<OfflineLecture> {
  return apiGetMessage(OfflineLectureSchema, `/offline/${packageId}`);
}

export async function completeOfflineDownload(packageId: number): Promise<void> {
  await apiPost<unknown>(`/offline/${packageId}/complete`, {});
}

export function deleteOfflineLecture(packageId: number): Promise<void> {
  return apiDelete(`/offline/${packageId}`);
}

/** The renewed expiry of each package, or null for one that was revoked. */
export async function syncOfflineLicenses(packageIds: number[]): Promise<Map<number, number | null>> {
  const res = await apiPostMessage(
    SyncOfflineLicensesRequestSchema,
    SyncOfflineLicensesResponseSchema,
    "/offline/sync",
    create(SyncOfflineLicensesRequestSchema, { packageIds }),
  );
  const out = new Map<number, number | null>();
  for (const l of res.licenses) {
    out.set(l.packageId, l.revoked || !l.expiresAt ? null : timestampDate(l.expiresAt).getTime());
  }
  return out;
}
