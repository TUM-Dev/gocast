export function setLocationCookie() {
    let currentDate = new Date(new Date().getTime() + 30*60000);
    document.cookie = "redirectURL=" + window.location.href + "; expires=" + currentDate + "; path=/"
}
