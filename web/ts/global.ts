async function postData(url = '', data = {}) {
    const response = await fetch(url, {
        method: 'POST',
        headers: {
            'Content-Type': 'application/json'
        },
        body: JSON.stringify(data)
    });
    return response
}

function Get(yourUrl) {
    let HttpReq = new XMLHttpRequest();
    HttpReq.open("GET", yourUrl, false);
    HttpReq.send(null);
    return HttpReq.responseText;
}

function showMessage(msg: string) {
    let alertBox: HTMLElement = document.getElementById("alertBox")
    let alertText: HTMLSpanElement = document.getElementById("alertText")
    alertText.innerText = msg
    alertBox.classList.remove("hidden")
}

function copyToClipboard(text: string) {
    const dummy = document.createElement("input");
    document.body.appendChild(dummy);
    dummy.value = text;
    dummy.select();
    document.execCommand("copy");
    document.body.removeChild(dummy);
}
