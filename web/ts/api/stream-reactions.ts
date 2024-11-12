import {getData, postData} from "../global";
import {get} from "../utilities/fetch-wrappers";


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