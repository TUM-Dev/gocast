class Watch {
    private chatInput: HTMLInputElement;
    private ws: WebSocket

    constructor() {
        (document.getElementById("chatForm") as HTMLFormElement).addEventListener("submit", e => this.submitChat(e))
        document.getElementById("chatBox").scrollTop = document.getElementById("chatBox").scrollHeight
        this.chatInput = document.getElementById("chatInput") as HTMLInputElement
        this.ws = new WebSocket("ws://localhost:8080/api/chat/" + (document.getElementById("streamID") as HTMLInputElement).value + "/ws")
        this.ws.onmessage = function (m) {
            let chatElem = document.createElement("div") as HTMLDivElement
            chatElem.classList.add("bg-secondary", "rounded", "p-2", "mx-2", "mb-2")
            chatElem.innerText = m.data
            document.getElementById("chatBox").appendChild(chatElem)
            document.getElementById("chatBox").scrollTop = document.getElementById("chatBox").scrollHeight
        }
    }

    submitChat(e: Event) {
        e.preventDefault()
        this.ws.send(this.chatInput.value)
        this.chatInput.value = ""
        return false//prevent form submission
    }
}

new Watch()