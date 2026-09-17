/**
 * Lecture halls, for the administration page.
 *
 * Camera preset management -- the grid of images fetched from a hall's camera,
 * refreshing it, marking a default and taking a new snapshot -- is not part of this
 * client. It still lives behind the v1 endpoints in api/lecture_halls.go; folding it
 * into v2 needs the CamService and the preset image directory wired into that API,
 * which is a bigger change than this page's migration.
 */

import type { LectureHallAdmin } from "@/gen/server/apiv2_pb";
import {
  CreateLectureHallAdminRequestSchema,
  LectureHallAdminSchema,
  ListLectureHallsAdminResponseSchema,
  UpdateLectureHallAdminRequestSchema,
} from "@/gen/server/apiv2_pb";
import { apiDelete, apiGetMessage, apiPatchMessage, apiPostMessage } from "./api";

/** 1 = rtsp, 2 = srt; mirrors model.StreamProtocol. */
export const enum StreamProtocol {
  RTSP = 1,
  SRT = 2,
}

export interface LectureHall {
  id: number;
  name: string;
  streamProtocol: StreamProtocol;
  combIp: string;
  presIp: string;
  camIp: string;
  cameraIp: string;
  pwrCtrlIp: string;
}

/** The fields a create or update request carries; the id is only in the latter. */
export interface LectureHallInput {
  name: string;
  streamProtocol: StreamProtocol;
  combIp: string;
  presIp: string;
  camIp: string;
  cameraIp: string;
  pwrCtrlIp: string;
}

function toLectureHall(lh: LectureHallAdmin): LectureHall {
  return {
    id: lh.id,
    name: lh.name,
    streamProtocol: lh.streamProtocol as StreamProtocol,
    combIp: lh.combIp,
    presIp: lh.presIp,
    camIp: lh.camIp,
    cameraIp: lh.cameraIp,
    pwrCtrlIp: lh.pwrCtrlIp,
  };
}

/** Every lecture hall, in whatever order the server returns them. */
export async function fetchLectureHalls(): Promise<LectureHall[]> {
  const res = await apiGetMessage(ListLectureHallsAdminResponseSchema, "/admin/lecture-halls");
  return res.lectureHalls.map(toLectureHall);
}

export async function createLectureHall(input: LectureHallInput): Promise<LectureHall> {
  const created = await apiPostMessage(
    CreateLectureHallAdminRequestSchema,
    LectureHallAdminSchema,
    "/admin/lecture-halls",
    { $typeName: "protobuf.CreateLectureHallAdminRequest", ...input },
  );
  return toLectureHall(created);
}

export async function updateLectureHall(id: number, input: LectureHallInput): Promise<LectureHall> {
  const updated = await apiPatchMessage(
    UpdateLectureHallAdminRequestSchema,
    LectureHallAdminSchema,
    `/admin/lecture-halls/${id}`,
    { $typeName: "protobuf.UpdateLectureHallAdminRequest", id, ...input },
  );
  return toLectureHall(updated);
}

export async function deleteLectureHall(id: number): Promise<void> {
  await apiDelete(`/admin/lecture-halls/${id}`);
}
