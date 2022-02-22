import { saveLectureName, saveLectureDescription } from "./edit-course";
import { Get } from "./global";
import { Calendar } from "@fullcalendar/core";
import dayGridPlugin from "@fullcalendar/daygrid";
import timeGridPlugin from "@fullcalendar/timegrid";
import iCalendarPlugin from "@fullcalendar/icalendar";

export function addScheduleListener() {
    document.addEventListener("DOMContentLoaded", function () {
        const calendarEl = document.getElementById("calendar");
        // Init fullcallendar
        const calendar = new Calendar(calendarEl, {
            plugins: [dayGridPlugin, timeGridPlugin, iCalendarPlugin],
            headerToolbar: { center: "timeGridDay,timeGridWeek" },
            initialView: "timeGridDay",
            nowIndicator: true,
            firstDay: 1,
            height: "85vh",
            allDaySlot: false,
            events: {
                url: "/api/hall/all.ics",
                format: "ics",
            },
            eventDidMount: function (e) {
                // manipulate dom element on event rendering -> inject events location
                e.el.title = e.event.title;
                const eventLocation = e.event.extendedProps.location;
                if (eventLocation !== null && eventLocation !== undefined && eventLocation !== "") {
                    e.el.title = e.el.title + " Location: " + eventLocation;
                    const locationElem = document.createElement("i");
                    locationElem.innerHTML = "&#183;" + eventLocation;
                    e.el.getElementsByClassName("fc-event-time")[0].appendChild(locationElem);
                }
            },
            eventClick: function (data) {
                // load some extra info on click
                const popover = document.getElementById("popoverContent");
                const streamInfo = JSON.parse(Get("/api/stream/" + data.event.extendedProps.description));
                popover.innerHTML = `
            <p class="flex text-1 text-lg">
                <span class="grow">${streamInfo["course"]}</span>
                <i id="closeBtn" class="transition-colors duration-200 hover:text-1 text-4 icon-close"></i>
            </p>
                <div class="text-2">
                    <div class="flex"><p>${new Date(streamInfo["start"]).toLocaleString()}</p></div>
                    <div class="flex"><span class="mr-2 font-semibold">Server: </span><p>${
                        streamInfo["ingest"]
                    }</p><i class="fas fa-copy ml-2 text-4 transition transition-colors hover:text-1" title="copy" onclick="copyToClipboard('${
                    streamInfo["ingest"]
                }')"></i></div>
                </div>
                <form onsubmit="saveLectureName(event, ${streamInfo["courseID"]}, ${streamInfo["streamID"]})"
                    class="w-full flex flex-row border-b-2 focus-within:border-gray-300 border-gray-500">
                    <label for="lectureNameInput${streamInfo["streamID"]}" class="hidden">Lecture title</label>
                    <input id="lectureNameInput${streamInfo["streamID"]}"
                        onfocus="focusNameInput(this, ${streamInfo["streamID"]})"
                        class="grow border-none" type="text" value="${streamInfo["name"]}"
                        placeholder="Lecture 2: Dark-Patterns I"
                        autocomplete="off">
                    <button id="nameSubmitBtn${streamInfo["streamID"]}"
                        class="fas fa-check ml-2 invisible text-gray-400 hover:text-purple-500"></button>
                </form>
                <form onsubmit="saveLectureDescription(event, ${streamInfo["courseID"]}, ${streamInfo["streamID"]})"
                    class="w-full flex flex-row border-b-2 focus-within:border-gray-300 border-gray-500">
                    <label for="lectureDescriptionInput${
                        streamInfo["streamID"]
                    }" class="hidden">Lecture description</label>
                    <textarea id="lectureDescriptionInput${streamInfo["streamID"]}"
                        rows="3"
                        onfocus="focusDescriptionInput(this, ${streamInfo["streamID"]})"
                        class="grow border-none"
                        placeholder="Add a nice description, links, and more. You can use Markdown."
                        autocomplete="off">${streamInfo["description"]}</textarea>
                    <button id="descriptionSubmitBtn${streamInfo["streamID"]}"
                        class="fas fa-check ml-2 invisible text-4 hover:text-1"></button>
                </form>
            <a class="text-3 hover:text-black dark:hover:text-white" href="/admin/course/${
                streamInfo["courseID"]
            }#lecture-li-${streamInfo["streamID"]}">Edit <i class="fas fa-external-link-alt"></i></a>
            `;
                document.getElementsByClassName("fc-timegrid").item(0)?.classList.add("filter", "blur-xxs");
                popover.classList.remove("hidden");
                document.getElementById("closeBtn").onclick = () => {
                    document.getElementsByClassName("fc-timegrid").item(0)?.classList.remove("filter", "blur-xxs");
                    popover.classList.add("hidden");
                    this.render();
                };
                this.render();
            },
        });
        calendar.render();
    });
}
