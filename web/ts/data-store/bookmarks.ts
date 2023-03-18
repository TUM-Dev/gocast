import {Delete, getData, postData, putData, Section, Time} from "../global";

export class BookmarksProvider {
    protected data: Map<string, Bookmark[]> = new Map<string, Bookmark[]>();

    async getData(streamId: number, forceFetch: boolean = false): Promise<Bookmark[]> {
        if (this.data[streamId] == null || forceFetch) {
            await this.fetch(streamId);
        }
        return this.data[streamId];
    }

    async fetch(streamId: number): Promise<void> {
        this.data[streamId] = (await Bookmarks.get(streamId)).map((b) => {
            b.streamId = streamId;
            b.friendlyTimestamp = new Time(b.hours, b.minutes, b.seconds).toString();
            return b;
        });
    }

    async add(request: AddBookmarkRequest): Promise<void> {
        await Bookmarks.add(request);
        await this.fetch(request.StreamID);
    }

    async update(bookmark: Bookmark, request: UpdateBookmarkRequest): Promise<Bookmark> {
        await Bookmarks.update(bookmark.ID, request);
        await this.fetch(bookmark.streamId);
        return this.data[bookmark.streamId].find((e) => e.ID === bookmark.ID);
    }

    async delete(bookmark: Bookmark): Promise<void> {
        await Bookmarks.delete(bookmark.ID);
        await this.fetch(bookmark.streamId);
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
