// changeVoDOrder Returns negated parameter and scrolls to top of the VoD list
function changeVoDOrder(order: boolean): boolean {
    const isAsc = !order;
    setTimeout(() => {
        if (isAsc) {
            // since 'flex-col-reverse' breaks the property we need to take a negative number large enough.
            document.getElementById("vod-list").scrollTop = -1000;
        }
    }, 10); // just enough to trigger alpine-js first
    return isAsc;
}