document.addEventListener("DOMContentLoaded", function () {
    const calendarEl = document.getElementById("calendar");
    // Init fullcallendar
    // @ts-ignore
    const calendar = new FullCalendar.Calendar(calendarEl, {
        initialView: "timeGridWeek",
        nowIndicator: true,
        firstDay: 1,
        height: "85vh",
        allDaySlot: false,
        events: {
            url: "/api/hall/all.ics",
            format: "ics"
        },
        eventDidMount: function (e) { // manipulate dom element on event rendering -> inject events location
            e.el.title = e.event.title;
            const eventLocation = e.event.extendedProps.location;
            if (eventLocation !== null && eventLocation !== undefined && eventLocation !== "") {
                e.el.title = e.el.title + " Location: " + eventLocation;
                const locationElem = document.createElement("i");
                locationElem.innerHTML = "&#183;" + eventLocation;
                e.el.getElementsByClassName("fc-event-time")[0].appendChild(locationElem);
            }
        },
        eventClick: function (data) { // load some extra info on click
            if (document.getElementById("popoverContent") !== null) { // don't rerender popup if there already is one
                return
            }
            const streamInfo = JSON.parse(Get("/api/stream/" + data.event.extendedProps.description))
            let html = `
            <div id="popoverContent" class="cursor-auto absolute top-full left-1/2 transform -translate-x-1/2 p-4 bg-secondary-lighter rounded z-10 border border-gray-500">
            <p class="flex text-white text-lg">
                <span class="flex-grow">${streamInfo["course"]}</span>
                <i id="closeBtn" class="transition-colors duration-200 hover:text-white text-gray-400 icon-close"></i>
            </p>
                <div class="text-gray-200">
                    <div class="flex"><p>${new Date(streamInfo["start"]).toLocaleString()}</p></div>
                    <div class="flex"><span class="mr-2 font-semibold">Key: </span><p>${streamInfo["key"]}</p><i class="fas fa-copy ml-2 text-gray-400 transition transition-colors hover:text-white" title="copy" onclick="copyToClipboard('${streamInfo['key']}')"></i></div>
                </div>
            </div>
            `;
            let newEl = document.createElement("div");
            newEl.innerHTML = html;
            data.el.parentElement.appendChild(newEl);
            // z index of parent must be larger than the max amount of events that overlap (10 should be plenty)
            data.el.parentElement.style.zIndex = 10;
            const c = this;
            document.getElementById("closeBtn").onclick = function () { // remove el and restore zIndex
                data.el.parentElement.style.zIndex = 1;
                document.getElementById('popoverContent').remove();
                c.render();
            };
            this.render()
        },
    });
    calendar.render();
});