/**
 * Lecture halls, for the administration page. Includes camera preset management --
 * the grid of images fetched from a hall's camera, refreshing it, marking a default
 * and taking a new snapshot.
 */

import { EmptySchema } from "@bufbuild/protobuf/wkt";

import type { CameraPresetAdmin, LectureHallAdmin } from "@/gen/server/apiv2_pb";
import {
  CameraPresetAdminSchema,
  CreateLectureHallAdminRequestSchema,
  LectureHallAdminSchema,
  ListLectureHallsAdminResponseSchema,
  RefreshLectureHallPresetsAdminRequestSchema,
  SetDefaultCameraPresetAdminRequestSchema,
  TakeCameraPresetSnapshotAdminRequestSchema,
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
  cameraPresets: CameraPreset[];
}

/** A camera preset. presetId is the camera's own numbering, not a database id. */
export interface CameraPreset {
  lectureHallId: number;
  presetId: number;
  name: string;
  /** Filename under the static directory; empty until a snapshot has been taken. */
  image: string;
  isDefault: boolean;
}

function toCameraPreset(preset: CameraPresetAdmin): CameraPreset {
  return {
    lectureHallId: preset.lectureHallId,
    presetId: preset.presetId,
    name: preset.name,
    image: preset.image,
    isDefault: preset.isDefault,
  };
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
    cameraPresets: lh.cameraPresets.map(toCameraPreset),
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

/**
 * Fetches the presets configured on a hall's camera and replaces the stored list
 * with them, returning the hall with its refreshed presets.
 */
export async function refreshLectureHallPresets(id: number): Promise<LectureHall> {
  const updated = await apiPostMessage(
    RefreshLectureHallPresetsAdminRequestSchema,
    LectureHallAdminSchema,
    `/admin/lecture-halls/${id}/presets/refresh`,
    { $typeName: "protobuf.RefreshLectureHallPresetsAdminRequest", id },
  );
  return toLectureHall(updated);
}

/** Marks one preset default for a hall; the server unsets the others. */
export async function setDefaultCameraPreset(lectureHallId: number, presetId: number): Promise<void> {
  await apiPostMessage(
    SetDefaultCameraPresetAdminRequestSchema,
    EmptySchema,
    `/admin/lecture-halls/${lectureHallId}/presets/${presetId}/default`,
    { $typeName: "protobuf.SetDefaultCameraPresetAdminRequest", lectureHallId, presetId },
  );
}

/**
 * Moves the camera to the preset, waits for it to arrive, and photographs it. Takes
 * several seconds -- the camera switch delay -- before it resolves.
 */
export async function takeCameraPresetSnapshot(
  lectureHallId: number,
  presetId: number,
): Promise<CameraPreset> {
  const updated = await apiPostMessage(
    TakeCameraPresetSnapshotAdminRequestSchema,
    CameraPresetAdminSchema,
    `/admin/lecture-halls/${lectureHallId}/presets/${presetId}/snapshot`,
    { $typeName: "protobuf.TakeCameraPresetSnapshotAdminRequest", lectureHallId, presetId },
  );
  return toCameraPreset(updated);
}
