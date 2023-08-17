import { ValueStream } from "../value-stream";
import { Course } from "../api/courses";
import { ChatMessage } from "../api/chat";

/**
 * Tunnels are an observer pattern based communication channel for multiple components
 */
export abstract class Tunnel {
    static pinned: ValueStream<PinnedUpdate> = new ValueStream();
    static reply: ValueStream<SetReply> = new ValueStream();
}

export interface PinnedUpdate {
    pin: boolean /* true if pinned, false if unpinned */;
    course: Course;
}

export interface SetReply {
    message: ChatMessage;
}
