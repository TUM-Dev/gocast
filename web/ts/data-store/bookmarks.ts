import {Delete, getData, postData, putData, Time} from "../global";
import {ValueStreamMap} from "./stream";

export class BookmarksProvider {
    protected data: Map<number, Bookmark[]> = new Map<number, Bookmark[]>();
    protected stream: ValueStreamMap<Bookmark[]> = new ValueStreamMap<Bookmark[]>();

    private triggerUpdate(streamId: number) {
        this.stream.add(streamId.toString(), this.data[streamId]);
    }

    private async fetch(streamId: number): Promise<void> {
        this.data[streamId] = (await Bookmarks.get(streamId)).map((b) => {
            b.streamId = streamId;
            b.friendlyTimestamp = new Time(b.hours, b.minutes, b.seconds).toString();
            return b;
        });
    }

    async subscribe(streamId: number, callback: BookmarkValueListener): Promise<void> {
        if (this.data[streamId] == null) {
            await this.fetch(streamId);
        }

        this.stream.subscribe(streamId.toString(), callback);
        this.triggerUpdate(streamId);
    }

    unsubscribe(streamId: number, callback: BookmarkValueListener): void {
        this.stream.unsubscribe(streamId.toString(), callback);
    }

    async getData(streamId: number, forceFetch: boolean = false): Promise<Bookmark[]> {
        if (this.data[streamId] == null || forceFetch) {
            await this.fetch(streamId);
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
            if (b.ID === bookmarkId) b.description = request.Description;
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

export type BookmarkValueListener = (value: Bookmark[]) => void;

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

export const Bookmarks = {
    get: async function (streamId: number): Promise<Bookmark[]> {
        return getData("/api/bookmarks?streamID=" + streamId)
            .then((resp) => {
                if (!resp.ok) {
                    throw Error(resp.statusText);
                }
                return resp.json();
            })
            .catch((err) => {
                console.error(err);
            })
            .then((j: Promise<Bookmark[]>) => j);
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
