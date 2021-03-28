class Watch {
    private chatInput: HTMLInputElement;
    private ws: WebSocket

    constructor() {
        (document.getElementById("chatForm") as HTMLFormElement).addEventListener("submit", e => this.submitChat(e))
        document.getElementById("chatBox").scrollTop = document.getElementById("chatBox").scrollHeight
        this.chatInput = document.getElementById("chatInput") as HTMLInputElement
        this.ws = new WebSocket("wss://live.mm.rbg.tum.de:443/api/chat/" + (document.getElementById("streamID") as HTMLInputElement).value + "/ws")
        this.ws.onmessage = function (m) {
            let chatElem = document.createElement("div") as HTMLDivElement
            chatElem.classList.add("bg-secondary", "rounded", "p-2", "mx-2", "mb-2")
            chatElem.innerText = m.data
            document.getElementById("chatBox").appendChild(chatElem)
            document.getElementById("chatBox").scrollTop = document.getElementById("chatBox").scrollHeight
        }
        if (document.getElementById("viewerCount") != null) {
            Watch.loadStat()
            setInterval(function () {
                Watch.loadStat()
            }, 60 * 1000)
        }
    }

    submitChat(e: Event) {
        e.preventDefault()
        this.ws.send(this.chatInput.value)
        this.chatInput.value = ""
        return false//prevent form submission
    }

    private static loadStat() {
        let stat = JSON.parse(Get("/api/chat/" + (document.getElementById("streamID") as HTMLInputElement).value + "/stats"))
        document.getElementById("viewerCount").innerText = stat["viewers"]
    }
}

new Watch()