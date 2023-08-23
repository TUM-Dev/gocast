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

export type Lecture = {
    ID?: number;
    courseId: number;
    courseSlug: string;
    lectureId: number;
    streamKey: string;
    seriesIdentifier: string;
    color: string;
    vodViews: number;
    start: Date;
    end: Date;
    isLiveNow: boolean;
    isConverting: boolean;
    isRecording: boolean;
    isPast: boolean;
    hasStats: boolean;
    name: string;
    description: string;
    lectureHallId: string;
    lectureHallName: string;
    isChatEnabled: false;
};

/**
 * REST API Wrapper for /api/stream/:id/sections
 */
export const AdminLectureList = {
    get: async function (courseId: number): Promise<Lecture[]> {
        return get(`/api/stream/${courseId}/sections`);
    },

    add: async function (courseId: number, request: object) {
        return post(`/api/stream/${courseId}/sections`, request);
    },

    update: async function (courseId: number, lectureId: number, request: UpdateLectureMetaRequest) {
        const promises = [];
        if (request.name !== null) {
            promises.push(postData("/api/course/" + courseId + "/renameLecture/" + lectureId, { name: request.name }));
        }

        if (request.description !== null) {
            promises.push(postData("/api/course/" + courseId + "/updateDescription/" + lectureId, { name: request.description }));
        }

        if (request.lectureHallId !== null) {
            promises.push(postData("/api/setLectureHall", { streamIds: [lectureId], lectureHall: request.lectureHallId }));
        }

        if (request.isChatEnabled !== null) {
            promises.push(patchData("/api/stream/" + lectureId + "/chat/enabled", { lectureId, isChatEnabled: request.isChatEnabled }));
        }

        const errors = (await Promise.all(promises)).filter((res) => res.status !== StatusCodes.OK);
        if (errors.length > 0) {
            console.error(errors);
            throw Error("Failed to update all data.");
        }
    },

    delete: async function (courseId: number, lectureId: number): Promise<Response> {
        return await postData(`/api/course/${courseId}/deleteLectures`, {
            streamIDs: [lectureId],
        });
    },
};
