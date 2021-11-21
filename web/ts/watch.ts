// @ts-nocheck
export module Watch {
    class Watch {
        private chatInput: HTMLInputElement;

        constructor() {
            if (document.getElementById("chatForm") != null) {
                const appHeight = () => {
                    const doc = document.documentElement;
                    doc.style.setProperty('--chat-height', `calc(${window.innerHeight}px - 5rem)`);
                }
                window.addEventListener('resize', appHeight);
                appHeight();
                document.getElementById("chatBox").scrollTop = document.getElementById("chatBox").scrollHeight
                this.chatInput = document.getElementById("chatInput") as HTMLInputElement
            }
        }

        submitChat(e: Event) {
            e.preventDefault();
            ws.send(JSON.stringify({
                "msg": this.chatInput.value,
                "anonymous": (document.getElementById("anonymous") as HTMLInputElement).checked
            }))
            this.chatInput.value = "";
            return false;//prevent form submission
        }
    }

    let ws: WebSocket;
    let retryInt = 5000; //retry connecting to websocket after this timeout
    let pageloaded = new Date();

    function startWebsocket() {
        let streamid = (document.getElementById("streamID") as HTMLInputElement).value;
        ws = new WebSocket("wss://live.rbg.tum.de/api/chat/" + streamid + "/ws");
        let cf = document.getElementById("chatForm");
        if (cf !== null && cf != undefined) {
            (document.getElementById("chatForm") as HTMLFormElement).addEventListener("submit", e => submitChat(e));
        }
        ws.onmessage = function (m) {
            const data = JSON.parse(m.data);
            if ("viewers" in data && document.getElementById("viewerCount") != null) {
                document.getElementById("viewerCount").innerText = data["viewers"];
            } else if ("live" in data) {
                window.location.reload();
            } else if ("paused" in data) {
                const paused: boolean = data["paused"];
                if (paused) {
                    //window.dispatchEvent(new CustomEvent("pausestart"))
                } else {
                    window.dispatchEvent(new CustomEvent("pauseend"));
                }
            } else if ("msg" in data) {
                const chatElem = createMessageElement(data);
                document.getElementById("chatBox").appendChild(chatElem);
                document.getElementById("chatBox").scrollTop = document.getElementById("chatBox").scrollHeight;
            }
        }

        ws.onclose = function () {
            // connection closed, discard old websocket and create a new one after backoff
            // don't recreate new connection if page has been loaded more than 12 hours ago
            if ((new Date()).valueOf() - pageloaded.valueOf() > 1000 * 60 * 60 * 12) {
                return;
            }
            ws = null;
            retryInt *= 2; // exponential backoff
            setTimeout(startWebsocket, retryInt);
        }
    }

    startWebsocket();

    /*
    while I'm not a fan of huge frontend frameworks, this is a good example why they can be useful.
    */
    function createMessageElement(m): HTMLDivElement {
        // Header:
        let chatElem = document.createElement("div") as HTMLDivElement
        chatElem.classList.add("rounded", "p-2", "mx-2")
        let chatHeader = document.createElement("div") as HTMLDivElement
        chatHeader.classList.add("flex", "flex-row")
        let chatNameField = document.createElement("p") as HTMLParagraphElement
        chatNameField.classList.add("flex-grow", "font-semibold")
        if (m["admin"]) {
            chatNameField.classList.add("text-warn")
        }
        chatNameField.innerText = m["name"]
        chatHeader.appendChild(chatNameField)

        const d = new Date
        d.setTime(Date.now())
        let chatTimeField = document.createElement("p") as HTMLParagraphElement
        chatTimeField.classList.add("text-4")
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

    function submitChat(e: Event) {
        e.preventDefault();
        ws.send(JSON.stringify({
            "msg": this.chatInput.value,
            "anonymous": (document.getElementById("anonymous") as HTMLInputElement).checked
        }))
        this.chatInput.value = "";
        return false;//prevent form submission
    }

    class Timer {

        constructor(date: string) {
            let d = new Date(date);
            d.setMinutes(d.getMinutes() - 10);
            this.countdown(d.getTime());
        }

        private countdown(countDownDate): void {
            // Update the count down every 1 second
            const x = setInterval(function () {

                // Get today's date and time
                const now = new Date().getTime();

                // Find the distance between now and the count down date
                const distance = countDownDate - now;

                // Time calculations for days, hours, minutes and seconds
                const days = Math.floor(distance / (1000 * 60 * 60 * 24));
                const hours = Math.floor((distance % (1000 * 60 * 60 * 24)) / (1000 * 60 * 60));
                const minutes = Math.floor((distance % (1000 * 60 * 60)) / (1000 * 60));
                const seconds = Math.floor((distance % (1000 * 60)) / 1000);

                // Display the result in the element with id="demo"
                let out = ""
                if (days === 1) {
                    out = "Live in one day";
                } else if (days > 1) {
                    out = "Live in " + days + " days";
                } else {
                    if (hours !== 0) {
                        out += hours.toLocaleString("en-US", {minimumIntegerDigits: 2}) + ":";
                    }
                    out += minutes.toLocaleString("en-US", {minimumIntegerDigits: 2}) + ":" + seconds.toLocaleString("en-US", {minimumIntegerDigits: 2})
                }
                document.getElementById("timer").innerText = out;

                // If the count down is finished, write some text
                if (distance < 0) {
                    clearInterval(x);
                    document.getElementById("timer").innerText = "";
                }
            }, 1000);
        }
    }

    new Watch()
}
