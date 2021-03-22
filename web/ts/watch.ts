class Watch {
    private player
    private chatInput: HTMLInputElement;

    constructor(player) {
        // @ts-ignore
        this.player = player
        (document.getElementById("chatForm") as HTMLFormElement).addEventListener("submit", e => this.submitChat(e))
        this.chatInput = document.getElementById("chatInput") as HTMLInputElement
        addEventListener('keypress', this.handleKeyPress)
    }

    submitChat(e: Event) {
        e.preventDefault()

    }

    private handleKeyPress(ev: KeyboardEvent): void {
        if (this.chatInput === document.activeElement) {
            return
        }
        switch (ev.key){
            case "f":
                this.player.requestFullScreen()
                break
            default : return;
        }
    }
}
