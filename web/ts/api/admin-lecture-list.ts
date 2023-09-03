import { del, get, post, put } from "../utilities/fetch-wrappers";
import {StatusCodes} from "http-status-codes";
import {patchData, postData, putData, UploadFile, UploadFileListener} from "../global";

export interface UpdateLectureMetaRequest {
    name?: string;
    description?: string;
    lectureHallId?: number;
    isChatEnabled?: boolean;
}

export interface CreateNewLectureRequest {
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

export interface LectureVideoType {
    key: string;
    type: string;
}

export const LectureVideoTypeComb = { key: "newCombinedVideo", type: "COMB" } as LectureVideoType;
export const LectureVideoTypePres = { key: "newPresentationVideo", type: "PRES" } as LectureVideoType;
export const LectureVideoTypeCam = { key: "newCameraVideo", type: "CAM" } as LectureVideoType;


export const LectureVideoTypes = [
    LectureVideoTypeComb,
    LectureVideoTypePres,
    LectureVideoTypeCam,
] as LectureVideoType[];

export interface Lecture {
    color:                 string;
    courseId:              number;
    courseSlug:            string;
    description:           string;
    downloadableVods:      DownloadableVOD[];
    end:                   string;
    files:                 null;
    hasStats:              boolean;
    isChatEnabled:         boolean;
    isConverting:          boolean;
    isLiveNow:             boolean;
    isPast:                boolean;
    isRecording:           boolean;
    lectureHallId:         number;
    lectureHallName:       string;
    lectureId:             number;
    name:                  string;
    private:               boolean;
    seriesIdentifier:      string;
    start:                 string;
    streamKey:             string;
    transcodingProgresses: any[];

    // Clientside computed fields
    hasAttachments: boolean;
    startDate: Date;
    startDateFormatted: string;
    startTimeFormatted: string;
    endDate: Date;
    endDateFormatted: string;
    endTimeFormatted: string;

    // Clientside pseudo fields
    newCombinedVideo: File|null;
    newPresentationVideo: File|null;
    newCameraVideo: File|null;
}

export interface DownloadableVOD {
    FriendlyName: string;
    DownloadURL:  string;
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
            promises.push(postData(`/api/course/${courseId}/renameLecture/${lectureId}`, { name: request.name }));
        }

        if (request.description !== undefined) {
            promises.push(putData(`/api/course/${courseId}/updateDescription/${lectureId}`, { name: request.description }));
        }

        if (request.lectureHallId !== undefined) {
            promises.push(postData("/api/setLectureHall", { streamIds: [lectureId], lectureHall: request.lectureHallId }));
        }

        if (request.isChatEnabled !== undefined) {
            promises.push(patchData(`/api/stream/${lectureId}/chat/enabled`, { lectureId, isChatEnabled: request.isChatEnabled }));
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
    saveSeriesMetadata: async (courseId: number, lectureId: number) => {
        return await postData(`/api/course/${courseId}/updateLectureSeries/${lectureId}`);
    },

    /**
     * Updates the private state of a lecture.
     * @param lectureId
     * @param isPrivate
     */
    setPrivate: async (lectureId: number, isPrivate: boolean): Promise<void> => {
        const res = await patchData(`/api/stream/${lectureId}/visibility`, { private: isPrivate });
        if (res.status !== StatusCodes.OK) {
            throw Error(res.body.toString());
        }
    },

    /**
     * Uploads a video to a lecture.
     * @param courseId
     * @param lectureId
     * @param videoType
     * @param file
     * @param listener
     */
    uploadVideo: async (courseId: number, lectureId: number, videoType: string, file: File, listener : UploadFileListener = {}) => {
        await UploadFile(
            `/api/course/${courseId}/uploadVODMedia?streamID=${lectureId}&videoType=${videoType}`,
            file,
            listener,
        );
    },

    /**
     * Deletes a lecture
     * @param courseId
     * @param lectureIds
     */
    delete: async function (courseId: number, lectureIds: number[]): Promise<Response> {
        return await postData(`/api/course/${courseId}/deleteLectures`, {
            streamIDs: lectureIds,
        });
    },
};
