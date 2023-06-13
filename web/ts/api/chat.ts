import { get } from "../utilities/fetch-wrappers";

class ChatMessage {
    readonly ID: number;
    readonly admin: boolean;

    readonly userId: number;
    readonly message: string;
    readonly name: string;
    readonly color: string;

    visible: boolean;
}

/**
 * REST API Wrapper for /api/chat
 */
export const ChatAPI = {
    async getMessages(streamId: number): Promise<ChatMessage[]> {
        return get(`/api/chat/${streamId}/messages`);
    },
};
