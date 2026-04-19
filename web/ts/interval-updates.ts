export function periodicCurrentTime(id: string) {
    const time = document.getElementById(id);
    setInterval(() => (time.innerHTML = new Date().toLocaleTimeString()), 1000);
}
