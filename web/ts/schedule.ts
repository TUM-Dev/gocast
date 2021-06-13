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
            let popover = document.getElementById("popoverContent");
            const streamInfo = JSON.parse(Get("/api/stream/" + data.event.extendedProps.description))
            let html = `
            <p class="flex text-white text-lg">
                <span class="flex-grow">${streamInfo["course"]}</span>
                <i id="closeBtn" class="transition-colors duration-200 hover:text-white text-gray-400 icon-close"></i>
            </p>
                <div class="text-gray-200">
                    <div class="flex"><p>${new Date(streamInfo["start"]).toLocaleString()}</p></div>
                    <div class="flex"><span class="mr-2 font-semibold">Key: </span><p>${streamInfo["key"]}</p><i class="fas fa-copy ml-2 text-gray-400 transition transition-colors hover:text-white" title="copy" onclick="copyToClipboard('${streamInfo['key']}')"></i></div>
                </div>
            `;
            popover.innerHTML = html;
            document.getElementsByClassName("fc-timegrid").item(0)?.classList.add("filter", "blur-xxs");
            popover.classList.remove("hidden")
            const c = this;
            document.getElementById("closeBtn").onclick = function () {
                document.getElementsByClassName("fc-timegrid").item(0)?.classList.remove("filter", "blur-xxs");
                popover.classList.add("hidden");
                c.render();
            };
            this.render()
        },
    });
    calendar.render();
});