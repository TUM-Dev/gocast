document.addEventListener("DOMContentLoaded", function () {
    const calendarEl = document.getElementById("calendar");
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
        eventDidMount: function (e) {
            e.el.title = e.event.title;
            const eventLocation = e.event.extendedProps.location;
            if (eventLocation !== undefined && eventLocation !== "") {
                e.el.title = e.el.title + " Location: " + eventLocation;
                const locationElem = document.createElement("i");
                locationElem.innerHTML = "&#183;" + eventLocation;
                e.el.getElementsByClassName("fc-event-time")[0].appendChild(locationElem);
            }
        },
        eventClick: function (data) {
            Get("/api/stream/"+data.event.extendedProps.description)
            const details = document.getElementById("popoverContent")
            details.classList.remove("hidden")
            const c = this;
            document.getElementById("closeBtn").onclick = function () {
                document.getElementById('popoverContent').classList.add('hidden');
                c.render();
            };
            this.render()
        },
    });
    calendar.render();
});