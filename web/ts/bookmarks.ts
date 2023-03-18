import { Delete, getData, postData, putData, Time } from "./global";
import { getPlayers } from "./TUMLiveVjs";
import {AddBookmarkRequest, Bookmark, Bookmarks, UpdateBookmarkRequest} from "./data-store/bookmarks";
import {DataStore} from "./data-store/data-store";

export class BookmarkList {
    private readonly streamId: number;

    private list: Bookmark[];

    constructor(streamId: number) {
        this.streamId = streamId;
    }

    get(): Bookmark[] {
        return this.list;
    }

    length(): number {
        return this.list !== undefined ? this.list.length : 0;
    }

    async delete(id: number) {
        await Bookmarks.delete(id).then(() => {
            const index = this.list.findIndex((b) => b.ID === id);
            this.list.splice(index, 1);
        });
    }

    async fetch() {
        this.list = await DataStore.bookmarks.getData(this.streamId);
    }
}

export class BookmarkDialog {
    private readonly streamId: number;

    request: AddBookmarkRequest;

    constructor(streamId: number) {
        this.streamId = streamId;
    }

    async submit() {
        // convert strings to number
        this.request.Hours = +this.request.Hours;
        this.request.Minutes = +this.request.Minutes;
        this.request.Seconds = +this.request.Seconds;
        await Bookmarks.add(this.request);
    }

    reset(): void {
        const player = getPlayers()[0];
        const time = Time.FromSeconds(player.currentTime()).toObject();
        this.request = {
            StreamID: this.streamId,
            Description: "",
            Hours: time.hours,
            Minutes: time.minutes,
            Seconds: time.seconds,
        };
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

    async submit() {
        await this.bookmark.update(this.request).then(() => (this.show = false));
    }

    reset() {
        this.show = false;
        this.request = new UpdateBookmarkRequest();
        this.request.Description = this.bookmark.description;
    }
}
