import { AlpineComponent } from "./alpine-component";
import { ChatAPI } from "../api/chat";

export function chatContext(): AlpineComponent {
    return {
        async init() {
            const messages = await ChatAPI.getMessages(12845);
            console.log(messages);
        },
    } as AlpineComponent;
}
