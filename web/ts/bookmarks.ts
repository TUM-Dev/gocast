import { Delete, getData, postData, putData } from "./global";
import { currentTimeToHMS } from "./TUMLiveVjs";

export class BookmarkList {
    private readonly streamId: number;

    private list: Bookmark[];
    showEdit: boolean;

    constructor(streamId: number) {
        this.streamId = streamId;
        this.showEdit = false;
    }

    get(): Bookmark[] {
        return this.list;
    }

    async delete(id: number) {
        await Bookmarks.delete(id).then(() => {
            const index = this.list.findIndex((b) => b.ID === id);
            this.list.splice(index, 1);
        });
    }

    length(): number {
        return this.list.length;
    }

    async fetch() {
        this.list = await Bookmarks.get(this.streamId);
    }
}

export class BookmarkDialog {
    private readonly streamId: number;

    request: AddBookmarkRequest;
    showSuccess: boolean;

    constructor(streamId: number) {
        this.streamId = streamId;
    }

    async submit(e: FormDataEvent) {
        e.preventDefault();
        await Bookmarks.add(this.request).then(() => (this.showSuccess = true));
    }

    reset(): void {
        const time = currentTimeToHMS();
        this.request = { StreamID: this.streamId, Description: "", Hours: time.h, Minutes: time.m, Seconds: time.s };
        this.showSuccess = false;
    }
}

export class BookmarkUpdater {
    private readonly old: Bookmark;

    request: UpdateBookmarkRequest;
    success: boolean;
    show: boolean;

    constructor(old: Bookmark) {
        this.old = old;
        this.request = new UpdateBookmarkRequest();
        this.request.Description = old.description;
        this.success = false;
        this.show = false;
    }

    async submit(e: FormDataEvent) {
        e.preventDefault();
        if (this.old.description !== this.request.Description) {
            await Bookmarks.update(this.old.ID, this.request).then(() => {
                this.success = true;
                this.show = false;
            });
        } else {
            this.show = false;
        }
    }
}

const Bookmarks = {
    get: async (streamId: number) => {
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
            .then((j) => j);
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

class AddBookmarkRequest {
    StreamID: number;
    Description: string;
    Hours: number;
    Minutes: number;
    Seconds: number;
}

class UpdateBookmarkRequest {
    Description: string;
}

type Bookmark = {
    ID: number;
    description: string;
    hours: number;
    minutes: number;
    seconds: number;
    friendlyTimestamp?: string;
};
