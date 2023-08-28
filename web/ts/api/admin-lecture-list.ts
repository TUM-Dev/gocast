import { del, get, post, put } from "../utilities/fetch-wrappers";
import {StatusCodes} from "http-status-codes";
import {patchData, postData} from "../global";

export type UpdateLectureMetaRequest = {
    name?: string;
    description?: string;
    lectureHallId?: number;
    isChatEnabled?: boolean;
}

export type CreateNewLectureRequest = {
    title: "",
    lectureHallId: 0,
    start: "",
    end: "",
    isChatEnabled: false,
    duration: 0, // Duration in Minutes
    formatedDuration: "", // Duration in Minutes
    premiere: false,
    vodup: false,
    adHoc: false,
    recurring: false,
    recurringInterval: "weekly",
    eventsCount: 10,
    recurringDates: [],
    combFile: [],
    presFile: [],
    camFile: [],
}

export interface Lecture {
    ID: number;
    CreatedAt: string;
    UpdatedAt: string;
    DeletedAt: string;
    Name: string;
    Description: string;
    CourseID: number;
    Start: string;
    End: string;
    ChatEnabled: boolean;
    RoomName: string;
    RoomCode: string;
    EventTypeName: string;
    TUMOnlineEventID: number;
    SeriesIdentifier: string;
    StreamKey: string;
    PlaylistUrl: string;
    PlaylistUrlPRES: string;
    PlaylistUrlCAM: string;
    LiveNow: boolean;
    LiveNowTimestamp: string;
    Recording: boolean;
    Premiere: boolean;
    Ended: boolean;
    VodViews: number;
    StartOffset: number;
    EndOffset: number;
    LectureHallID: number;
    ThumbInterval: number;
    StreamName: string;
    Duration: LectureDuration;
    //VideoSections?: null;
    Private: boolean;
    Watched: boolean;

    // Clientside computed fields
    startDate: Date;
    startDateFormatted: string;
    startTimeFormatted: string;
    endDate: Date;
    endDateFormatted: string;
    endTimeFormatted: string;
}

export interface LectureDuration {
    Int32: number;
    Valid: boolean;
}

/**
 * REST API Wrapper for /api/stream/:id/sections
 */
export const AdminLectureList = {
    get: async function (courseId: number): Promise<Lecture[]> {
        const result = await get(`/api/course/${courseId}/lectures`);
        return result["streams"];
    },

    add: async function (courseId: number, request: object) {
        return post(`/api/stream/${courseId}/sections`, request);
    },

    // TODO: make one unified endpoint
    update: async function (courseId: number, lectureId: number, request: UpdateLectureMetaRequest) {
        const promises = [];
        if (request.name !== undefined) {
            promises.push(postData("/api/course/" + courseId + "/renameLecture/" + lectureId, { name: request.name }));
        }

        if (request.description !== undefined) {
            promises.push(postData("/api/course/" + courseId + "/updateDescription/" + lectureId, { name: request.description }));
        }

        if (request.lectureHallId !== undefined) {
            promises.push(postData("/api/setLectureHall", { streamIds: [lectureId], lectureHall: request.lectureHallId }));
        }

        if (request.isChatEnabled !== undefined) {
            promises.push(patchData("/api/stream/" + lectureId + "/chat/enabled", { lectureId, isChatEnabled: request.isChatEnabled }));
        }

        const errors = (await Promise.all(promises)).filter((res) => res.status !== StatusCodes.OK);
        if (errors.length > 0) {
            console.error(errors);
            throw Error("Failed to update all data.");
        }
    },

    delete: async function (courseId: number, lectureIds: number[]): Promise<Response> {
        return await postData(`/api/course/${courseId}/deleteLectures`, {
            streamIDs: lectureIds,
        });
    },
};
