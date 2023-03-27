import { Delete, getData, postData, putData, Section, Time } from "../global";
import { StreamableMapProvider } from "./provider";
import {Cache} from "./cache";
import {Bookmark} from "./bookmarks";

export class VideoSectionProvider extends StreamableMapProvider<number, Section[]> {
    async getData(streamId: number, forceFetch = false): Promise<Section[]> {
        if (this.data[streamId] == null || forceFetch) {
            await this.fetch(streamId);
        }
        return this.data[streamId];
    }

    async fetch(streamId: number): Promise<void> {
        this.data[streamId] = (await VideoSections.get(streamId)).map((s) => {
            s.friendlyTimestamp = new Time(s.startHours, s.startMinutes, s.startSeconds).toString();
            return s;
        });
    }

    async add(streamId: number, sections: Section[]): Promise<void> {
        await VideoSections.add(streamId, sections);
        await this.fetch(streamId);
        this.triggerUpdate(streamId);
    }

    async delete(streamId: number, sectionId: number) {
        await VideoSections.delete(streamId, sectionId);
        this.data[streamId] = (await this.getData(streamId)).filter((s) => s.ID !== sectionId);
    }

    async update(streamId: number, sectionId: number, request: UpdateVideoSectionRequest) {
        await VideoSections.update(streamId, sectionId, request);
        this.data[streamId] = (await this.getData(streamId)).map((s) => {
            if (s.ID === sectionId) {
                s = {
                    ...s,
                    startHours: request.StartHours,
                    startMinutes: request.StartMinutes,
                    startSeconds: request.StartSeconds,
                    description: request.Description,
                    friendlyTimestamp: new Time(
                        request.StartHours,
                        request.StartMinutes,
                        request.StartSeconds,
                    ).toString(),
                };
            }
            return s;
        });
        this.triggerUpdate(streamId);
    }
}

export class UpdateVideoSectionRequest {
    Description: string;
    StartHours: number;
    StartMinutes: number;
    StartSeconds: number;
}

/**
 * Wrapper for REST-API calls @ /api/stream/:id/sections
 * @category watch-page
 * @category admin-page
 */
const VideoSections = {
    cache: new Cache<Bookmark[]>({ validTime: 1000 }),

    get: async function (streamId: number, forceCacheRefresh: boolean = false): Promise<Section[]> {
        return this.cache.get(`get.${streamId}`, async () => {
            const resp = await getData(`/api/stream/${streamId}/sections`);
            if (!resp.ok) {
                throw Error(resp.statusText);
            }
            return resp.json();
        }, forceCacheRefresh);
    },

    add: async function (streamId: number, request: object) {
        return postData(`/api/stream/${streamId}/sections`, request);
    },

    update: function (streamId: number, id: number, request: UpdateVideoSectionRequest) {
        return putData(`/api/stream/${streamId}/sections/${id}`, request);
    },

    delete: async function (streamId: number, id: number): Promise<Response> {
        return Delete(`/api/stream/${streamId}/sections/${id}`);
    },
};
