import { AlpineComponent } from "./alpine-component";
import { User } from "../api/users";

enum InteractionType {
    Chat,
    Polls,
}

export function watchInteractionContext(user: User) {
    return {
        type: InteractionType.Chat,
        user: user as User,

        init() {
            console.log("init watchInteraction");
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
    } as AlpineComponent;
}
