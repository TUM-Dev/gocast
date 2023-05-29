import { post } from "../utilities/fetch-wrappers";

export const WatchedAPI = {
    async update(streamID: number, watched: boolean) {
        return post("/api/watched", { streamID, watched });
    },
};
