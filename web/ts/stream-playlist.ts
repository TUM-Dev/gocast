import { DataStore } from "./data-store/data-store";
import { StreamPlaylistEntry } from "./data-store/stream-playlist";

export class StreamPlaylist {
    private streamId: number;
    private elem: HTMLElement;
    private list: StreamPlaylistEntry[];

    protected constructor(streamId: number, element: HTMLElement) {
        this.streamId = streamId;
        this.elem = element;
        this.list = [];
        DataStore.streamPlaylist.subscribe(this.streamId, (data) => this.onUpdate(data));
    }

    private onUpdate(data: StreamPlaylistEntry[]) {
        this.list = data.filter((item) => !item.liveNow && (new Date(item.start).getTime()) < (new Date().getTime()));

        const { prev, next } = this.findNextAndPrev();
        this.elem.dispatchEvent(new CustomEvent("update", { detail: { list: this.list, prev, next } }));

        setTimeout(() => {
            this.elem.querySelector(".--selected").scrollIntoView({ block: "center" });
        }, 10);
    }

    private findNextAndPrev(): { next: StreamPlaylistEntry; prev: StreamPlaylistEntry } {
        const streamIndex = this.list.findIndex((e) => e.streamId == this.streamId);
        const prevIndex = streamIndex - 1 >= 0 ? streamIndex - 1 : null;
        const nextIndex = streamIndex + 1 < this.list.length ? streamIndex + 1 : null;
        return { prev: this.list[prevIndex], next: this.list[nextIndex] };
    }
}
