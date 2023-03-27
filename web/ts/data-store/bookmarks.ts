import { Delete, getData, postData, putData, Time } from "../global";
import { StreamableMapProvider } from "./provider";
import { Cache } from "./cache";

export class BookmarksProvider extends StreamableMapProvider<number, Bookmark[]> {
    protected async fetch(streamId: number, force: boolean = false): Promise<void> {
        this.data[streamId] = (await Bookmarks.get(streamId, force)).map((b) => {
            b.streamId = streamId;
            b.friendlyTimestamp = new Time(b.hours, b.minutes, b.seconds).toString();
            return b;
        });
    }

    async getData(streamId: number, forceFetch = false): Promise<Bookmark[]> {
        if (this.data[streamId] == null || forceFetch) {
            await this.fetch(streamId, forceFetch);
            this.triggerUpdate(streamId);
        }
        return this.data[streamId];
    }

    async add(request: AddBookmarkRequest): Promise<void> {
        await Bookmarks.add(request);
        await this.fetch(request.StreamID);
        this.triggerUpdate(request.StreamID);
    }

    async update(streamId: number, bookmarkId: number, request: UpdateBookmarkRequest): Promise<void> {
        await Bookmarks.update(bookmarkId, request);
        this.data[streamId] = (await this.getData(streamId)).map((b) => {
            if (b.ID === bookmarkId) b = { ...b, description: request.Description };
            return b;
        });
        this.triggerUpdate(streamId);
    }

    async delete(streamId: number, bookmarkId: number): Promise<void> {
        await Bookmarks.delete(bookmarkId);
        this.data[streamId] = (await this.getData(streamId)).filter((b) => b.ID !== bookmarkId);
        this.triggerUpdate(streamId);
    }
}

export type Bookmark = {
    ID: number;
    streamId: number;
    description: string;
    hours: number;
    minutes: number;
    seconds: number;
    friendlyTimestamp?: string;
};

export class AddBookmarkRequest {
    StreamID: number;
    Description: string;
    Hours: number;
    Minutes: number;
    Seconds: number;
}

export class UpdateBookmarkRequest {
    Description: string;
}

const Bookmarks = {
    cache: new Cache<Bookmark[]>({ validTime: 1000 }),

    get: async function (streamId: number, forceCacheRefresh: boolean = false): Promise<Bookmark[]> {
        return this.cache.get(`get.${streamId}`, async () => {
            const resp = await getData("/api/bookmarks?streamID=" + streamId);
            if (!resp.ok) {
                throw Error(resp.statusText);
            }
           return resp.json();
        }, forceCacheRefresh);
    },

    add: (request: AddBookmarkRequest) => {
        return postData("/api/bookmarks", request)
            .then((resp) => {
                if (!resp.ok) {
                    throw Error(resp.statusText);
                }
            })
            .catch((err) => {
                console.error(err);
            });
    },

    update: (bookmarkId: number, request: UpdateBookmarkRequest) => {
        return putData("/api/bookmarks/" + bookmarkId, request)
            .then((resp) => {
                if (!resp.ok) {
                    throw Error(resp.statusText);
                }
            })
            .catch((err) => {
                console.error(err);
            });
    },

    delete: (bookmarkId: number) => {
        return Delete("/api/bookmarks/" + bookmarkId)
            .then((resp) => {
                if (!resp.ok) {
                    throw Error(resp.statusText);
                }
            })
            .catch((err) => {
                console.error(err);
            });
    },
};
