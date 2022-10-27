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

    length(): number {
        return this.list.length;
    }

    async delete(id: number) {
        await Bookmarks.delete(id).then(() => {
            const index = this.list.findIndex((b) => b.ID === id);
            this.list.splice(index, 1);
        });
    }

    async fetch() {
        this.list = await Bookmarks.get(this.streamId);
        this.list.forEach((b) => (b.update = updateBookmark));
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
        // convert strings to number
        this.request.Hours = +this.request.Hours;
        this.request.Minutes = +this.request.Minutes;
        this.request.Seconds = +this.request.Seconds;
        console.log(this.request);
        await Bookmarks.add(this.request).then(() => (this.showSuccess = true));
    }

    reset(): void {
        const time = currentTimeToHMS();
        this.request = { StreamID: this.streamId, Description: "", Hours: time.h, Minutes: time.m, Seconds: time.s };
        this.showSuccess = false;
    }
}

export class BookmarkUpdater {
    private readonly bookmark: Bookmark;

    request: UpdateBookmarkRequest;
    show: boolean;

    constructor(b: Bookmark) {
        this.bookmark = b;
        this.reset();
    }

    async submit(e: FormDataEvent) {
        e.preventDefault();
        await this.bookmark.update(this.request).then(() => (this.show = false));
    }

    reset() {
        this.request = new UpdateBookmarkRequest();
        this.request.Description = this.bookmark.description;
        this.show = false;
    }
}

type Bookmark = {
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

const Bookmarks = {
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
