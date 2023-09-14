import {del, get, patch, post, put} from "../utilities/fetch-wrappers";
import { StatusCodes } from "http-status-codes";
import { Delete, patchData, postData, postFormData, putData, uploadFile, UploadFileListener } from "../global";

export interface UpdateLectureMetaRequest {
    name?: string;
    description?: string;
    lectureHallId?: number;
    isChatEnabled?: boolean;
}

export class LectureFile {
    readonly id: number;
    readonly fileType: number;
    readonly friendlyName: string;

    constructor({ id, fileType, friendlyName }) {
        this.id = id;
        this.fileType = fileType;
        this.friendlyName = friendlyName;
    }
}

export interface CreateNewLectureRequest {
    title: "";
    lectureHallId: 0;
    start: "";
    end: "";
    isChatEnabled: false;
    duration: 0; // Duration in Minutes
    formatedDuration: ""; // Duration in Minutes
    premiere: false;
    vodup: false;
    adHoc: false;
    recurring: false;
    recurringInterval: "weekly";
    eventsCount: 10;
    recurringDates: [];
    combFile: [];
    presFile: [];
    camFile: [];
}

export interface TranscodingProgress {
    version: string;
    progress: number;
}

export interface LectureVideoType {
    key: string;
    type: string;
}

export const LectureVideoTypeComb = {
    key: "newCombinedVideo",
    type: "COMB",
} as LectureVideoType;

export const LectureVideoTypePres = {
    key: "newPresentationVideo",
    type: "PRES",
} as LectureVideoType;

export const LectureVideoTypeCam = {
    key: "newCameraVideo",
    type: "CAM",
} as LectureVideoType;

export const LectureVideoTypes = [
    LectureVideoTypeComb,
    LectureVideoTypePres,
    LectureVideoTypeCam,
] as LectureVideoType[];

export type VideoSection = {
    id?: number;
    description: string;

    startHours: number;
    startMinutes: number;
    startSeconds: number;

    streamID?: number;
    fileID?: number;
};

export function videoSectionTimestamp(a: VideoSection) : number {
    return a.startHours * 3600 + a.startMinutes * 60 + a.startSeconds;
}

export function videoSectionSort(a: VideoSection, b: VideoSection) : number {
    return videoSectionTimestamp(a) - videoSectionTimestamp(b);
}

export interface Lecture {
    color: string;
    courseId: number;
    courseSlug: string;
    description: string;
    downloadableVods: DownloadableVOD[];
    end: string;
    files: LectureFile[];
    hasStats: boolean;
    isChatEnabled: boolean;
    isConverting: boolean;
    isLiveNow: boolean;
    isPast: boolean;
    isRecording: boolean;
    lectureHallId: number;
    lectureHallName: string;
    lectureId: number;
    name: string;
    private: boolean;
    seriesIdentifier: string;
    start: string;
    streamKey: string;
    transcodingProgresses: TranscodingProgress[];
    videoSections: VideoSection[];

    // Clientside computed fields
    hasAttachments: boolean;
    startDate: Date;
    startDateFormatted: string;
    startTimeFormatted: string;
    endDate: Date;
    endDateFormatted: string;
    endTimeFormatted: string;

    // Clientside pseudo fields
    newCombinedVideo: File | null;
    newPresentationVideo: File | null;
    newCameraVideo: File | null;
}

export interface DownloadableVOD {
    FriendlyName: string;
    DownloadURL: string;
}

/**
 * REST API Wrapper for /api/stream/:id/sections
 */
export const AdminLectureList = {
    /**
     * Fetches all lectures for a course
     * @param courseId
     */
    get: async function (courseId: number): Promise<Lecture[]> {
        const result = await get(`/api/course/${courseId}/lectures`);
        return result["streams"];
    },

    /**
     * Adds a new lecture to a course.
     * @param courseId
     * @param request
     */
    add: async function (courseId: number, request: object) {
        return post(`/api/stream/${courseId}/sections`, request);
    },

    /**
     * Updates metadata of a lecture.
     * @param courseId
     * @param lectureId
     * @param request
     */
    updateMetadata: async function (courseId: number, lectureId: number, request: UpdateLectureMetaRequest) {
        const promises = [];
        if (request.name !== undefined) {
            promises.push(
                post(`/api/course/${courseId}/renameLecture/${lectureId}`, {
                    name: request.name,
                }),
            );
        }

        if (request.description !== undefined) {
            promises.push(
                put(`/api/course/${courseId}/updateDescription/${lectureId}`, {
                    name: request.description,
                }),
            );
        }

        if (request.lectureHallId !== undefined) {
            promises.push(
                post("/api/setLectureHall", {
                    streamIds: [lectureId],
                    lectureHall: request.lectureHallId,
                }),
            );
        }

        if (request.isChatEnabled !== undefined) {
            promises.push(
                patch(`/api/stream/${lectureId}/chat/enabled`, {
                    lectureId,
                    isChatEnabled: request.isChatEnabled,
                }),
            );
        }

        const errors = (await Promise.all(promises)).filter((res) => res.status !== StatusCodes.OK);
        if (errors.length > 0) {
            console.error(errors);
            throw Error("Failed to update all data.");
        }
    },

    /**
     * Distributes the lecture metadata of given lecture to all lectures in its series.
     * @param courseId
     * @param lectureId
     */
    saveSeriesMetadata: async (courseId: number, lectureId: number): Promise<void>  => {
        await post(`/api/course/${courseId}/updateLectureSeries/${lectureId}`);
    },

    /**
     * Add sections to a lecture
     * @param lectureId
     * @param sections
     */
    addSections: async (lectureId: number, sections: VideoSection[]): Promise<void> => {
        await post(`/api/stream/${lectureId}/sections`, sections);
    },

    /**
     * Updates a section
     * @param lectureId
     * @param section
     */
    updateSection: async (lectureId: number, section: VideoSection): Promise<void> => {
        const res = await putData(`/api/stream/${lectureId}/sections/${section.id}`, {
            Description: section.description,
            StartHours: section.startHours,
            StartMinutes: section.startMinutes,
            StartSeconds: section.startSeconds,
        });
        if (res.status !== StatusCodes.OK) {
            throw Error(res.body.toString());
        }
    },

    /**
     * Remove a section from a lecture
     * @param lectureId
     * @param sectionId
     */
    removeSection: async (lectureId: number, sectionId: number): Promise<void> => {
        await del(`/api/stream/${lectureId}/sections/${sectionId}`);
    },

    /**
     * Updates the private state of a lecture.
     * @param lectureId
     * @param isPrivate
     */
    setPrivate: async (lectureId: number, isPrivate: boolean): Promise<void> => {
        await patch(`/api/stream/${lectureId}/visibility`, {
            private: isPrivate,
        });
    },

    /**
     * Uploads a video to a lecture.
     * @param courseId
     * @param lectureId
     * @param videoType
     * @param file
     * @param listener
     */
    uploadVideo: async (
        courseId: number,
        lectureId: number,
        videoType: string,
        file: File,
        listener: UploadFileListener = {},
    ) => {
        await uploadFile(
            `/api/course/${courseId}/uploadVODMedia?streamID=${lectureId}&videoType=${videoType}`,
            file,
            listener,
        );
    },

    /**
     * Upload a file as attachment for a lecture
     * @param courseId
     * @param lectureId
     * @param file
     * @param listener
     */
    uploadAttachmentFile: async (
        courseId: number,
        lectureId: number,
        file: File,
        listener: UploadFileListener = {},
    ) => {
        return await uploadFile(`/api/stream/${lectureId}/files?type=file`, file, listener);
    },

    /**
     * Upload a url as attachment for a lecture
     * @param courseId
     * @param lectureId
     * @param url
     * @param listener
     */
    uploadAttachmentUrl: async (
        courseId: number,
        lectureId: number,
        url: string,
        listener: UploadFileListener = {},
    ) => {
        const vodUploadFormData = new FormData();
        vodUploadFormData.append("file_url", url);
        return postFormData(`/api/stream/${lectureId}/files?type=url`, vodUploadFormData, listener);
    },

    deleteAttachment: async (courseId: number, lectureId: number, attachmentId: number) => {
        return del(`/api/stream/${lectureId}/files/${attachmentId}`);
    },

    /**
     * Get transcoding progress
     * @param courseId
     * @param lectureId
     * @param version
     */
    getTranscodingProgress: async (courseId: number, lectureId: number, version: number): Promise<number> => {
        return (await fetch(`/api/course/${courseId}/stream/${lectureId}/transcodingProgress?v=${version}`)).json();
    },

    /**
     * Deletes a lecture
     * @param courseId
     * @param lectureIds
     */
    delete: async function (courseId: number, lectureIds: number[]): Promise<Response> {
        return await post(`/api/course/${courseId}/deleteLectures`, {
            streamIDs: lectureIds,
        });
    },
};
