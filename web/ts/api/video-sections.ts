import { del, get, post, put } from "../utilities/fetch-wrappers";

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
};
