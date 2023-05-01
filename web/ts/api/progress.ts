import { get } from "../utilities/fetch-wrappers";

export type ProgressDTO = Progress[];

export class Progress {
    private readonly progress: number;
    private readonly watched: boolean;
    private readonly streamId: number;

    constructor(obj: Progress) {
        this.progress = obj.progress;
        this.watched = obj.watched;
        this.streamId = obj.streamId;
    }

    public Percentage(): number {
        return Math.round(this.progress * 100);
    }

    public HasProgressOne(): boolean {
        return this.progress === 1;
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
