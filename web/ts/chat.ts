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
