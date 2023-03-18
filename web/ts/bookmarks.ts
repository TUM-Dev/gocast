import { Delete, getData, postData, putData, Time } from "./global";
import { getPlayers } from "./TUMLiveVjs";
import {AddBookmarkRequest, Bookmark, UpdateBookmarkRequest} from "./data-store/bookmarks";
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
        await DataStore.bookmarks.delete(this.streamId, id);
        await this.fetch();
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
        await DataStore.bookmarks.add(this.request);
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
        const updated = await DataStore.bookmarks.update(this.bookmark, this.request);
        this.bookmark.description = updated.description;
        this.show = false
    }

    reset() {
        this.show = false;
        this.request = new UpdateBookmarkRequest();
        this.request.Description = this.bookmark.description;
    }
}
