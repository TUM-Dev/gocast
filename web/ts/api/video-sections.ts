import { del, get, post, put } from "../utilities/fetch-wrappers";

export class UpdateVideoSectionRequest {
    Description: string;
    StartHours: number;
    StartMinutes: number;
    StartSeconds: number;
}

export type Section = {
    ID?: number;
    description: string;

    startHours: number;
    startMinutes: number;
    startSeconds: number;

    streamID: number;
    friendlyTimestamp?: string;
    fileID?: number;

    isCurrent: boolean;
};

/**
 * REST API Wrapper for /api/stream/:id/sections
 */
export const VideoSectionAPI = {
    get: async function (streamId: number): Promise<Section[]> {
        return get(`/api/stream/${streamId}/sections`);
    },

    add: async function (streamId: number, request: object) {
        return post(`/api/stream/${streamId}/sections`, request);
    },

    update: function (streamId: number, id: number, request: UpdateVideoSectionRequest) {
        return put(`/api/stream/${streamId}/sections/${id}`, request);
    },

    delete: async function (streamId: number, id: number): Promise<Response> {
        return del(`/api/stream/${streamId}/sections/${id}`);
    },
};
