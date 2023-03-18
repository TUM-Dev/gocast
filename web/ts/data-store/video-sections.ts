import {Delete, getData, postData, putData, Section, Time} from "../global";

export class VideoSectionProvider {
    protected data: Map<string, Section[]> = new Map<string, Section[]>();

    async getData(streamId: number, forceFetch: boolean = false) : Promise<Section[]> {
        if (this.data[streamId] == null || forceFetch) {
            await this.fetch(streamId);
        }
        return this.data[streamId];
    }

    async fetch(streamId: number) : Promise<void> {
        this.data[streamId] = (await VideoSections.get(streamId)).map((s) => {
            s.friendlyTimestamp = new Time(s.startHours, s.startMinutes, s.startSeconds).toString();
            return s;
        });
    }
}

/**
 * Wrapper for REST-API calls @ /api/stream/:id/sections
 * @category watch-page
 * @category admin-page
 */
export const VideoSections = {
    get: async function (streamId: number): Promise<Section[]> {
        return getData(`/api/stream/${streamId}/sections`)
            .then((resp) => {
                if (!resp.ok) {
                    throw Error(resp.statusText);
                }
                return resp.json();
            })
            .catch((err) => {
                console.error(err);
                return [];
            })
            .then((l: Section[]) => l);
    },

    add: async function (streamId: number, request: object) {
        return postData(`/api/stream/${streamId}/sections`, request);
    },

    update: function (streamId: number, id: number, request: object) {
        return putData(`/api/stream/${streamId}/sections/${id}`, request);
    },

    delete: async function (streamId: number, id: number): Promise<Response> {
        return Delete(`/api/stream/${streamId}/sections/${id}`);
    },
};
