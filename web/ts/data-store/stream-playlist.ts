import { Delete, getData, postData, putData, Time } from "../global";
import { StreamableMapProvider } from "./provider";

export class StreamPlaylistProvider extends StreamableMapProvider<number, StreamPlaylistEntry[]> {
    protected async fetcher(streamId: number): Promise<StreamPlaylistEntry[]> {
        const result = await StreamPlaylist.get(streamId);
        return result
            .map((e) => {
                e.createdAtDate = new Date(e.createdAt);
                return e;
            })
            .sort((a, b) => a.createdAtDate.getTime() - b.createdAtDate.getTime());
    }
}

export type StreamPlaylistEntry = {
    streamId: number;
    courseSlug: string;
    streamName: string;
    liveNow: boolean;
    watched: boolean;
    start: string;
    createdAt: string;

    // Client Generated
    createdAtDate: Date;
};

const StreamPlaylist = {
    get: async function (streamId: number): Promise<StreamPlaylistEntry[]> {
        const resp = await getData(`/api/stream/${streamId}/playlist`);
        if (!resp.ok) {
            throw Error(resp.statusText);
        }
        return resp.json();
    },
};
