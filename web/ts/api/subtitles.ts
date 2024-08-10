import { del, get, post, put } from "../utilities/fetch-wrappers";

export type Subtitle = {
    ID?: number;
    content: string;
};

/**
 * REST API Wrapper for /api/stream/:id/sections
 */
export const SubtitleAPI = {
    getPresent: async function (streamId: number): Promise<boolean> {
        return get(`/api/stream/${streamId}/subtitles/de`).then((res) => res.ok);
    },
};
