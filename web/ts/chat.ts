/*
    Returns 'chatOpen' value from localStorage or defaults with false
*/
function initChat() {
    const val = window.localStorage.getItem("chatOpen");
    return val ? JSON.parse(val) : false;
}

/*
    Saves negated show value in localStorage with key 'chatOpen'
    and returns the value.
 */
function toggleChat(show: boolean) {
    const neg = !show;
    window.localStorage.setItem("chatOpen", JSON.stringify(neg));
    return neg;
}
