import { get } from "../utilities/fetch-wrappers";
import { WatchedAPI } from "./watched";

export type ProgressDTO = Progress[];

export class Progress {
    private readonly progress: number;
    private watched: boolean;
    private readonly streamId: number;

    constructor(obj: Progress) {
        this.progress = obj.progress;
        this.watched = obj.watched;
        this.streamId = obj.streamId;
    }

    public get Watched() {
        return this.watched;
    }

    public async ToggleWatched(watched?: boolean) {
        this.watched = watched ? watched : !this.watched;
        return WatchedAPI.update(this.streamId, this.watched);
    }

    public Percentage(): number {
        return this.watched ? 100 : Math.round(this.progress * 100);
    }

    public HasProgressOne(): boolean {
        return this.progress === 1 || this.watched;
    }
}

/**
 * REST API Wrapper for /api/progress
 */
export const ProgressAPI = {
    getBatch(ids: number[]) {
        const query = "[]ids=" + ids.join("&[]ids=");
        return get("/api/progress/streams?" + query).then((progresses: ProgressDTO) => {
            return progresses.map((p) => new Progress(p)); // Recreate for Percentage(),...
        });
    },
};
