import { TopEmojis, Emoji } from "top-twitter-emojis-map";
/*
    Returns 'chatOpen' value from localStorage or defaults with false.
    Calls 'scrollToBottom' after 250ms, so that the 'chatBox' is already
    visible.
*/
export function initChat() {
    const val = window.localStorage.getItem("chatOpen");
    if (val) {
        setTimeout(scrollToBottom, 250);
    }
    return val ? JSON.parse(val) : false;
}

export function getTopEmojis() {
    return TopEmojis;
}

/*
    Scroll to the bottom of the 'chatBox'
 */
export function scrollToBottom() {
    document.getElementById("chatBox").scrollTop = document.getElementById("chatBox").scrollHeight;
}

/*
    Scroll to top of the 'chatBox'
 */
export function scrollToTop() {
    document.getElementById("chatBox").scrollTo({ top: 0, behavior: "smooth" });
}

/*
    Saves negated show value in localStorage with key 'chatOpen'
    and returns the value.
    Calls 'scrollToBottom' after 250ms, so that the 'chatBox' is already
    visible.
 */
export function toggleChat(show: boolean) {
    const neg = !show;
    if (neg) {
        setTimeout(scrollToBottom, 250);
    }
    window.localStorage.setItem("chatOpen", JSON.stringify(neg));
    return neg;
}

export function findEmojisForInput(input: string): Emoji[] {
    return TopEmojis.filter((emoji) => {
        return emoji.short_names.some((key) => key.startsWith(input));
    }).slice(0, 7);
}

/**
 * get currently typed word based on position in the input.
 * e.g.: "hello{cursor} world" => "hello" ([0, 4])
 */
function getCurrentWordPositions(input: string, cursorPos: number): [number, number] {
    const cursorStart = cursorPos;
    while (cursorPos > 0 && input.charAt(cursorPos - 1) !== " ") {
        cursorPos--;
    }
    return [cursorPos, cursorStart];
}

export async function getEmojisForMessage(message: string, cursorPos: number): Promise<Emoji[]> {
    const pos = getCurrentWordPositions(message, cursorPos);
    const currentWord = message.substring(pos[0], pos[1]);
    if (!currentWord.startsWith(":") || currentWord.length < 2) {
        return [];
    }
    return findEmojisForInput(currentWord.substring(1));
}

export function insertEmoji(emoji: Emoji) {
    const chatInput: HTMLInputElement = document.getElementById("chatInput") as HTMLInputElement;
    const pos = getCurrentWordPositions(chatInput.value, chatInput.selectionStart);
    // send new message to alpine
    window.dispatchEvent(
        new CustomEvent("setmessage", {
            detail: chatInput.value.substring(0, pos[0]) + emoji.emoji + " " + chatInput.value.substring(pos[1]),
        }),
    );
    chatInput.focus();
    chatInput.selectionStart = pos[0] + emoji.emoji.length + 1; // +1 for space
    chatInput.selectionEnd = pos[0] + emoji.emoji.length + 1;
    // notify alpine to remove emoji suggestions
    window.dispatchEvent(new CustomEvent("emojisinserted"));
}

let orderByLikes = false; // sorting by likes or by time

export function sortMessages(messages): void {
    messages.sort((m1, m2) => {
        if (orderByLikes) {
            if (m1.likes === m2.likes) {
                return m2.id - m1.id; // same amount of likes -> newer messages up
            }
            return m2.likes - m1.likes; // more likes -> up
        } else {
            return m1.ID < m2.ID ? -1 : 1; // newest messages last
        }
    });
}

export function setOrder(obl: boolean) {
    orderByLikes = obl;
}

export function shouldScroll(): boolean {
    if (orderByLikes) {
        return false; // only scroll if sorting by time
    }
    const c = document.getElementById("chatBox");
    return c.scrollHeight - c.scrollTop <= c.offsetHeight;
}

export function showNewMessageIndicator() {
    window.dispatchEvent(new CustomEvent("messageindicator", { detail: { show: true } }));
}

export function scrollChat() {
    if (orderByLikes) {
        return; // only scroll if sorting by time
    }
    const c = document.getElementById("chatBox");
    c.scrollTop = c.scrollHeight;
}

export function scrollToLatestMessage() {
    const c = document.getElementById("chatBox");
    c.scrollTo({ top: c.scrollHeight, behavior: "smooth" });
    window.dispatchEvent(new CustomEvent("messageindicator", { detail: { show: false } }));
}

export function showDisconnectedMsg() {
    if (document.getElementById("disconnectMsg") !== null) {
        document.getElementById("disconnectMsg").classList.remove("hidden");
    }
}

export function hideDisconnectedMsg() {
    if (document.getElementById("disconnectMsg") !== null) {
        document.getElementById("disconnectMsg").classList.add("hidden");
    }
}
