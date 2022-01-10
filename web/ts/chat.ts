/*
    Returns 'chatOpen' value from localStorage or defaults with false
*/
function initChat() {
    const val = window.localStorage.getItem("chatOpen");
    const node = document.getElementById("chatBox");
    const cb = function (mutationsList, observer) {
        for (const mutation of mutationsList) {
            if (mutation.attributeName === "style") {
                scrollToBottom();
            }
        }
    };
    const observer = new MutationObserver(cb);
    observer.observe(node, {attributes: true});
    return val ? JSON.parse(val) : false;
}

/*
    Scroll to the bottom of the 'chatBox'
 */
function scrollToBottom() {
    document.getElementById("chatBox").scrollTop = document.getElementById("chatBox").scrollHeight;
}

/*
    Saves negated show value in localStorage with key 'chatOpen'
    and returns the value.
 */
function toggleChat(show: boolean) {
    const neg = !show;
    if (neg) {
        setTimeout(scrollToBottom, 250);
    }
    window.localStorage.setItem("chatOpen", JSON.stringify(neg));
    return neg;
}
