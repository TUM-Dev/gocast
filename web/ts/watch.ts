class Watch {
    private chatInput: HTMLInputElement;
    private ws: WebSocket

    constructor() {
        this.ws = new WebSocket("ws://localhost:8080/api/chat/" + (document.getElementById("streamID") as HTMLInputElement).value + "/ws")
        if (document.getElementById("chatForm") != null) {
            (document.getElementById("chatForm") as HTMLFormElement).addEventListener("submit", e => this.submitChat(e))
            document.getElementById("chatBox").scrollTop = document.getElementById("chatBox").scrollHeight
            this.chatInput = document.getElementById("chatInput") as HTMLInputElement
            this.ws.onmessage = function (m) {
                const data = JSON.parse(m.data)
                if ("viewers" in data){
                    document.getElementById("viewerCount").innerText=data["viewers"]
                }else {
                    const chatElem = Watch.createMessageElement(data)
                    document.getElementById("chatBox").appendChild(chatElem)
                    document.getElementById("chatBox").scrollTop = document.getElementById("chatBox").scrollHeight
                }
            }
        }
    }

    submitChat(e: Event) {
        e.preventDefault()
        this.ws.send(JSON.stringify({
            "msg": this.chatInput.value,
            "anonymous": (document.getElementById("anonymous") as HTMLInputElement).checked
        }))
        this.chatInput.value = ""
        return false//prevent form submission
    }

    /*
    while I'm not a fan of huge frontend frameworks, this is a good example why they can be useful.
     */
    private static createMessageElement(m): HTMLDivElement {
        // Header:
        let chatElem = document.createElement("div") as HTMLDivElement
        chatElem.classList.add("rounded", "p-2", "mx-2")
        let chatHeader = document.createElement("div") as HTMLDivElement
        chatHeader.classList.add("flex", "flex-row")
        let chatNameField = document.createElement("p") as HTMLParagraphElement
        chatNameField.classList.add("flex-grow", "font-semibold")
        if (m["admin"]){
            chatNameField.classList.add("text-warn")
        }
        chatNameField.innerText = m["name"]
        chatHeader.appendChild(chatNameField)

        const d = new Date
        d.setTime(Date.now())
        let chatTimeField = document.createElement("p") as HTMLParagraphElement
        chatTimeField.classList.add("text-gray-500", "font-thin")
        chatTimeField.innerText = ("0" + d.getHours()).slice(-2) + ":" + ("0" + d.getMinutes()).slice(-2)
        chatHeader.appendChild(chatTimeField)
        chatElem.appendChild(chatHeader)

        // Message:
        let chatMessage = document.createElement("p") as HTMLParagraphElement
        chatMessage.classList.add("text-gray-300", "break-words")
        chatMessage.innerText = m["msg"]
        chatElem.appendChild(chatMessage)
        return chatElem
    }
}

new Watch()