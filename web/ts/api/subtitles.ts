import { del, get, post, put } from "../utilities/fetch-wrappers";

export type Subtitle = {
    ID: number;
    content: string;
};

/**
 * REST API Wrapper for /api/stream/:id/sections
 */
export const SubtitleAPI = {
    get: async function (streamId: number): Promise<Subtitle> {
        let str = await fetch(`/api/stream/${streamId}/subtitles/de`);
        console.log(str)
        if(str.ok && str.text.startsWith("{")) {
            return null;
        } else {
            return {ID: 0, content: str}
        }
    },
};
