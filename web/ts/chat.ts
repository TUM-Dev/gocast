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

export interface Emoji {
    k: string[];
    v: string;
}

export function findEmojisForInput(input: string): Emoji[] {
    return CHAT_EMOJIS.filter((emoji) => {
        return emoji.k.some((key) => key.startsWith(input));
    }).slice(0, 7);
}

const CHAT_EMOJIS: Emoji[] = [
    { k: ["100"], v: "💯" },
    { k: ["fire"], v: "🔥" },
    { k: ["+1", "thumbsup"], v: "👍" },
    { k: ["alien"], v: "👽" },
    { k: ["angry"], v: "😠" },
    { k: ["anguished"], v: "😧" },
    { k: ["astronished"], v: "😲" },
    { k: ["blush"], v: "😊" },
    { k: ["clown"], v: "🤡" },
    { k: ["cold_sweat"], v: "😰" },
    { k: ["confounded"], v: "😖" },
    { k: ["confused"], v: "😕" },
    { k: ["cowboy"], v: "🤠" },
    { k: ["cry"], v: "😢" },
    { k: ["disappointed"], v: "😞" },
    { k: ["disappointed_relieved"], v: "😥" },
    { k: ["dizzy_face"], v: "😵" },
    { k: ["drool"], v: "🤤" },
    { k: ["exploding_head"], v: "🤯" },
    { k: ["expressionless"], v: "😑" },
    { k: ["eyes"], v: "👀" },
    { k: ["face_vomiting"], v: "🤮" },
    { k: ["face_with_hand_over_mouth"], v: "🤭" },
    { k: ["face_with_monocle"], v: "🧐" },
    { k: ["face_with_raised_eyebrow"], v: "🤨" },
    { k: ["fearful"], v: "😨" },
    { k: ["flushed"], v: "😳" },
    { k: ["frowning"], v: "😦" },
    { k: ["frowning_2"], v: "☹️" },
    { k: ["ghost"], v: "👻" },
    { k: ["grimacing"], v: "😬" },
    { k: ["grin"], v: "😁" },
    { k: ["grinning"], v: "😀" },
    { k: ["head_bandage"], v: "🤕" },
    { k: ["heart_eyes"], v: "😍" },
    { k: ["hugging"], v: "🤗" },
    { k: ["hushed"], v: "😯" },
    { k: ["imp"], v: "👿" },
    { k: ["innocent"], v: "😇" },
    { k: ["jack_o_lantern"], v: "🎃" },
    { k: ["japanese_goblin"], v: "👺" },
    { k: ["japanese_ogre"], v: "👹" },
    { k: ["joy"], v: "😂" },
    { k: ["kissing"], v: "😗" },
    { k: ["kissing_closed_eyes"], v: "😚" },
    { k: ["kissing_heart"], v: "😘" },
    { k: ["kissing_smiling_eyes"], v: "😙" },
    { k: ["laughing"], v: "😆" },
    { k: ["liar"], v: "🤥" },
    { k: ["mask"], v: "😷" },
    { k: ["money_mouth"], v: "🤑" },
    { k: ["nerd"], v: "🤓" },
    { k: ["neutral_face"], v: "😐" },
    { k: ["no_mouth"], v: "😶" },
    { k: ["open_mouth"], v: "😮" },
    { k: ["pensive"], v: "😔" },
    { k: ["persevere"], v: "😣" },
    { k: ["poop"], v: "💩" },
    { k: ["rage"], v: "😡" },
    { k: ["relaxed"], v: "☺️" },
    { k: ["relieved"], v: "😌" },
    { k: ["robot"], v: "🤖" },
    { k: ["rofl"], v: "🤣" },
    { k: ["rolling_eyes"], v: "🙄" },
    { k: ["scream"], v: "😱" },
    { k: ["shushing_face"], v: "🤫" },
    { k: ["sick"], v: "🤢" },
    { k: ["skull"], v: "💀" },
    { k: ["skull_crossbones"], v: "☠️" },
    { k: ["sleeping"], v: "😴" },
    { k: ["sleepy"], v: "😪" },
    { k: ["slight_frown"], v: "🙁" },
    { k: ["slight_smile"], v: "🙂" },
    { k: ["smile"], v: "😄" },
    { k: ["smiley"], v: "😃" },
    { k: ["smiling_imp"], v: "😈" },
    { k: ["smirk"], v: "😏" },
    { k: ["sneeze"], v: "🤧" },
    { k: ["sob"], v: "😭" },
    { k: ["space_invader"], v: "👾" },
    { k: ["star_struck"], v: "🤩" },
    { k: ["stuck_out_tounge"], v: "😛" },
    { k: ["stuck_out_tounge_closed_eyes"], v: "😝" },
    { k: ["stuck_out_tounge_winking_eye"], v: "😜" },
    { k: ["sunglasses"], v: "😎" },
    { k: ["swearing"], v: "🤬" },
    { k: ["sweat"], v: "😓" },
    { k: ["sweat_smile"], v: "😅" },
    { k: ["thermometer_face"], v: "🤒" },
    { k: ["thinking"], v: "🤔" },
    { k: ["tired_face"], v: "😫" },
    { k: ["triumph"], v: "😤" },
    { k: ["unamused"], v: "😒" },
    { k: ["upside_down"], v: "🙃" },
    { k: ["weary"], v: "😩" },
    { k: ["wink"], v: "😉" },
    { k: ["worried"], v: "😟" },
    { k: ["yum"], v: "😋" },
    { k: ["zany_face"], v: "🤪" },
    { k: ["zipper_mouth"], v: "🤐" },
];

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
            detail: chatInput.value.substring(0, pos[0]) + emoji.v + " " + chatInput.value.substring(pos[1]),
        }),
    );
    chatInput.focus();
    chatInput.selectionStart = pos[0] + emoji.v.length + 1; // +1 for space
    chatInput.selectionEnd = pos[0] + emoji.v.length + 1;
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
