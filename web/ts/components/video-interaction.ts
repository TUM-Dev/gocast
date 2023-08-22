import { AlpineComponent } from "./alpine-component";
import { User } from "../api/users";
import { SocketConnections } from "../api/chat-ws";
import { RealtimeFacade } from "../utilities/ws";

enum InteractionType {
    Chat,
    Polls,
}

export function videoInteractionContext(user: User) {
    return {
        type: InteractionType.Chat,
        user: user as User,

        init() {
            SocketConnections.ws.subscribe();
        },

        showChat() {
            this.type = InteractionType.Chat;
        },

        isChat(): boolean {
            return this.type === InteractionType.Chat;
        },

        showPolls() {
            this.type = InteractionType.Polls;
        },

        isPolls(): boolean {
            return this.type === InteractionType.Polls;
        },

        isLoggedIn(): boolean {
            return this.user.ID !== 0;
        },

        isAdmin(): boolean {
            return this.user.isAdmin;
        },

        isPopOut(): boolean {
            return window.location.href.includes("/chat/popup");
        },
    } as AlpineComponent;
}
