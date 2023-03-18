import {Delete, getData, postData, putData, Section, Time} from "../global";

export class BookmarksProvider {
    protected data: Map<string, Bookmark[]> = new Map<string, Bookmark[]>();

    async getData(streamId: number, forceFetch: boolean = false) : Promise<Bookmark[]> {
        if (this.data[streamId] == null || forceFetch) {
            await this.fetch(streamId);
        }
        return this.data[streamId];
    }

    async fetch(streamId: number) : Promise<void> {
        this.data[streamId] = (await Bookmarks.get(streamId)).map((b) => {
            b.update = updateBookmark;
            b.friendlyTimestamp = new Time(b.hours, b.minutes, b.seconds).toString();
            return b;
        });
    }
}

export type Bookmark = {
    ID: number;
    description: string;
    hours: number;
    minutes: number;
    seconds: number;
    friendlyTimestamp?: string;

    update?: (UpdateBookmarkRequest) => Promise<void>;
};

async function updateBookmark(request: UpdateBookmarkRequest): Promise<void> {
    // this = Bookmark object
    if (this.description !== request.Description) {
        return await Bookmarks.update(this.ID, request).then(() => {
            this.description = request.Description;
        });
    }
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