import { DataStore } from "./data-store/data-store";
import {StreamPlaylistEntry} from "./data-store/stream-playlist";

export class StreamPlaylist {
    private streamId: number;
    private elem: HTMLElement;
    protected list: StreamPlaylistEntry[];

    protected constructor(streamId: number, element: HTMLElement) {
        this.streamId = streamId;
        this.elem = element;
        this.list = [];
        DataStore.streamPlaylist.subscribe(this.streamId, (data) => this.onUpdate(data));
    }

    private onUpdate(data: StreamPlaylistEntry[]) {
        this.list = data;
        this.elem.dispatchEvent(new CustomEvent("update", { detail: this.list }));
    }
}
