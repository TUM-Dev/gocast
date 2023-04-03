import { Delete, getData, postData, putData, Time } from "../global";
import { StreamableMapProvider } from "./provider";

export class BookmarksProvider extends StreamableMapProvider<number, Bookmark[]> {
    protected async fetcher(streamId: number): Promise<Bookmark[]> {
        const result = await Bookmarks.get(streamId);
        return result.map((b) => {
            b.streamId = streamId;
            b.friendlyTimestamp = new Time(b.hours, b.minutes, b.seconds).toString();
            return b;
        });
    }

    async add(request: AddBookmarkRequest): Promise<void> {
        await Bookmarks.add(request);
        await this.fetch(request.StreamID);
        await this.triggerUpdate(request.StreamID);
    }

    async update(streamId: number, bookmarkId: number, request: UpdateBookmarkRequest): Promise<void> {
        await Bookmarks.update(bookmarkId, request);
        this.data[streamId] = (await this.getData(streamId)).map((b) => {
            if (b.ID === bookmarkId) b = { ...b, description: request.Description };
            return b;
        });
        await this.triggerUpdate(streamId);
    }

    async delete(streamId: number, bookmarkId: number): Promise<void> {
        await Bookmarks.delete(bookmarkId);
        this.data[streamId] = (await this.getData(streamId)).filter((b) => b.ID !== bookmarkId);
        await this.triggerUpdate(streamId);
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
    get: async function (streamId: number): Promise<Bookmark[]> {
        const resp = await getData("/api/bookmarks?streamID=" + streamId);
        if (!resp.ok) {
            throw Error(resp.statusText);
        }
        return resp.json();
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
