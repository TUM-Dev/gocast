import {getData, postData} from "../global";
import {get} from "../utilities/fetch-wrappers";
import {Realtime, RealtimeMessageTypes} from "../socket";


// Function to add a reaction to a stream
export function addReaction(reaction: string, streamID: number) {
    return postData(`/api/stream/${streamID}/reaction`, { reaction: reaction });
}

// Function to get all possible reactions for a stream
export function getAllowedReactions(streamID: number): Promise<string[]> {
    return get(`/api/stream/${streamID}/reaction/allowed`).then((data) => {
        return data;
    });
}

export const liveReactionListener = {
    async init(streamId: string) {
        await Realtime.get().subscribeChannel("reaction-update", this.handle);
        await Realtime.get().send("reaction-update", {type: RealtimeMessageTypes.RealtimeMessageTypeChannelMessage, payload: {"streamID": streamId}});
    },

    handle(payload: object) {
        window.dispatchEvent(new CustomEvent("reactionupdate", { detail: { data: payload } }));
    },
};

export function startReactionAnimation(reaction: string) {
    const reactionElement = document.getElementById(`reaction-${reaction}`);
    if (reactionElement) {
        reactionElement.classList.add("opacity-0");
        setTimeout(() => {
            reactionElement.classList.remove("opacity-0");
        }, 500);
    }
}
