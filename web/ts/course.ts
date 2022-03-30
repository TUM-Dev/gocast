export function reorderVodList() {
    const vodList = document.getElementById("vod-list");
    const months = vodList.children;

    for (let i = months.length - 1; i >= 0; i--) {
        vodList.appendChild(months[i]);
    }
}
