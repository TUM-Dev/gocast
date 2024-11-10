export function periodicCurrentTime(id : string) {
    let time = document.getElementById(id);
    setInterval(() => time.innerHTML = new Date().toLocaleTimeString(), 1000);
}