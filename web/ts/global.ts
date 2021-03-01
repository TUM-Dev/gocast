async function postData(url = '', data = {}) {
    const response = await fetch(url, {
        method: 'POST',
        headers: {
            'Content-Type': 'application/json'
        },
        body: JSON.stringify(data)
    });
    return response.status
}

function showMessage(msg: string) {
    let alertBox: HTMLElement = document.getElementById("alertBox")
    let alertText: HTMLSpanElement = document.getElementById("alertText")
    alertText.innerText = msg
    alertBox.classList.remove("hidden")
}