import { StatusCodes } from "http-status-codes";
import { postData, showMessage } from "./global";

type UserStream = {
    streamID: number;
    month: string;
    progress: number;
    watched: boolean;
};

export function watchedTracker(): { m: WatchedTracker } {
    return { m: new WatchedTracker() };
}

export class WatchedTracker {
    streams: UserStream[] = [];
    courseId: number;
    userId: number;

    init(userStreams: UserStream[]) {
        this.streams = userStreams;
    }

    setWatched(streamId: number, watchStatus: boolean): void {
        const stream = this.streams.find((s) => {
            return s.streamID === streamId;
        });

        stream.watched = watchStatus;
        postData(`/api/watched`, { streamID: streamId, watched: watchStatus }).then((response: Response) => {
            if (response.status !== StatusCodes.OK) {
                showMessage("There was an error setting a recording watched: " + response.body);
            }
        });
    }

    userWatchedMonth(month: string): boolean {
        const unwatchedStreamIndex = this.streams.filter((s) => s.month === month).findIndex((s) => !s.watched);
        return unwatchedStreamIndex === -1;
    }

    userWatchedAll(): boolean {
        return this.streams.findIndex((s) => !s.watched) === -1;
    }
}
