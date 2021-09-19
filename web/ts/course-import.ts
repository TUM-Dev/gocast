const d = {'step': 0, 'lecturehall': '', 'loading': false, 'from': new Date(), 'to': (new Date()).setUTCDate((new Date()).getUTCDate() + 365)};

function pageData() {
    return d;
}

// lecture hall selected -> api call
window.addEventListener("notify1", evt => {
    fetch("/api/schedule/1").then(res => {
        console.log(res);
    })
})
