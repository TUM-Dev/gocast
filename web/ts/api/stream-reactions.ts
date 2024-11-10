import {getData, postData} from "../global";


// Function to add a reaction to a stream
export function addReaction(reaction: string, streamID: number) {
    return postData(`/api/${streamID}/reactions`, { reaction: reaction });
}

export function getAllowedReactions(streamID: number) {
    return getData(`/api/${streamID}/reactions/allowed`).then((data) => {
        return data;
    });
}