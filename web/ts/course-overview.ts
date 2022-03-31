import { StatusCodes } from "http-status-codes";
import { postData } from "./global";

class UserStream {
    streamID: number;
    month: string;
    progress: number;
    watched: boolean;
}

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
        console.log(this.streams);
        const stream = this.streams.find((s) => {
            console.log(s.streamID);
            console.log(streamId);
            return s.streamID === streamId;
        });

        console.log(stream);
        stream.watched = watchStatus;
        postData(`/api/markWatched`, { streamID: streamId, watched: watchStatus }).then((response: Response) => {
            if (response.status !== StatusCodes.OK) {
                console.log("Error marking stream watched");
            }
        });
    }

    userWatchedMonth(month: string): boolean {
        const unwatchedStreamIndex = this.streams.filter((s) => s.month === month).findIndex((s) => !s.watched);
        return unwatchedStreamIndex === -1;
    }

    userWatchedAll(): boolean {
        for (let i = 0; i < this.streams.length; i++) {
            if (!this.streams[i].watched) {
                return false;
            }
        }
        return true;
    }
}
