import { Course, CoursesAPI, Stream } from "../api/courses";
import { ProgressAPI } from "../api/progress";
import { Paginator } from "../utilities/paginator";
import { HasPinnedCourseDTO, UserAPI } from "../api/users";
import { copyToClipboard } from "../utilities/input-interactions";
import { AlpineComponent } from "./alpine-component";
import { Tunnel } from "../utilities/tunnels";
import { ToggleableElement } from "../utilities/ToggleableElement";
import { getFromStorage, setInStorage } from "../utilities/storage";
import { GroupedSmartArray, SmartArray } from "../utilities/smartarray";

export enum StreamSortMode {
    NewestFirst,
    OldestFirst,
}

export enum StreamFilterMode {
    ShowWatched,
    HideWatched,
}

export enum ViewMode {
    Grid,
    List,
}

export enum GroupMode {
    Month,
    Week,
}

export function courseContext(slug: string, year: number, term: string, userId: number): AlpineComponent {
    return {
        userId: userId as number,

        slug: slug as string,
        year: year as number,
        term: term as string,

        course: new Course() as Course,

        courseStreams: new GroupedSmartArray<Stream, number>(),
        plannedStreams: new Paginator<Stream>([], 3),
        upcomingStreams: new Paginator<Stream>([], 3),

        streamSortMode: +(getFromStorage("streamSortMode") ?? StreamSortMode.NewestFirst),
        streamFilterMode: +(getFromStorage("streamFilterMode") ?? StreamFilterMode.ShowWatched),
        viewMode: +(getFromStorage("viewMode") ?? ViewMode.Grid) as number,
        groupMode: +(getFromStorage("groupMode") ?? GroupMode.Month) as number,

        dateOfFirstWeek: new Date(),
        weekCountWithoutEmptyWeeks: new Map<number, number>(),
        groupNames: new Map<number, string>(),

        /**
         * AlpineJS init function which is called automatically in addition to 'x-init'
         */
        init() {
            this.reload(this.slug, this.year, this.term);
        },

        /**
         * (Re-)Load course context
         */
        reload(slug: string, year: number, term: string) {
            this.slug = slug;
            this.year = year;
            this.term = term;
            this.loadCourse()
                .catch((err) => {
                    if (err.message === "Unauthorized") {
                        document.location.href = "/login";
                    } else {
                        document.location.href = `/?year=${year}&term=${term}`; // redirect to start page on error
                    }
                })
                .then(() => {
                    this.loadPinned();
                    this.plannedStreams.set(this.course.Planned.reverse()).reset();
                    this.upcomingStreams.set(this.course.Upcoming).reset();
                    this.loadProgresses(this.course.Recordings.map((s: Stream) => s.ID))
                        .then((progresses) => {
                            this.course.Recordings.forEach((s: Stream, i) => (s.Progress = progresses[i]));
                        })
                        .then(() => this.initializeWeekMap())
                        .then(() => this.applyGroupView());
                    console.log("🌑 init course", this.course);
                });
        },

        /**
         * Return compare function for two streams
         * @param  {StreamSortMode} sortMode Sorting mode
         * @return Lambda function that compares two streams based on their .Start property
         */
        sortFn(sortMode: StreamSortMode) {
            return sortMode === StreamSortMode.NewestFirst
                ? (a: Stream, b: Stream) => a.CompareStart(b)
                : (a: Stream, b: Stream) => a.CompareStart(b) * -1;
        },

        filterPred(filterMode: StreamFilterMode) {
            return filterMode === StreamFilterMode.ShowWatched
                ? (_: Stream) => true
                : (s: Stream) => !s.Progress.Watched;
        },

        sortNewestFirst() {
            this.streamSortMode = StreamSortMode.NewestFirst;
            setInStorage("streamSortMode", StreamSortMode.NewestFirst.toString());
        },

        isNewestFirst(): boolean {
            return this.streamSortMode === StreamSortMode.NewestFirst.valueOf();
        },

        sortOldestFirst() {
            this.streamSortMode = StreamSortMode.OldestFirst;
            setInStorage("streamSortMode", StreamSortMode.OldestFirst.toString());
        },

        isOldestFirst(): boolean {
            return this.streamSortMode === StreamSortMode.OldestFirst.valueOf();
        },

        toggleShowWatched() {
            this.streamFilterMode =
                this.streamFilterMode === StreamFilterMode.ShowWatched
                    ? StreamFilterMode.HideWatched
                    : StreamFilterMode.ShowWatched;
            setInStorage("streamFilterMode", this.streamFilterMode.toString());
        },

        isHideWatched() {
            return this.streamFilterMode === StreamFilterMode.HideWatched;
        },

        toggleListView() {
            this.viewMode = this.viewMode === ViewMode.Grid ? ViewMode.List : ViewMode.Grid;
            setInStorage("viewMode", this.viewMode.toString());
        },

        isListView() {
            return this.viewMode == ViewMode.List;
        },

        toggleWeekView() {
            this.groupMode = this.groupMode === GroupMode.Month ? GroupMode.Week : GroupMode.Month;
            setInStorage("groupMode", this.groupMode.toString());
            this.applyGroupView();
        },

        isWeekView() {
            return this.groupMode == GroupMode.Week;
        },

        applyGroupView() {
            if (this.groupMode === GroupMode.Month) {
                this.courseStreams.set(this.course.Recordings, (s: Stream) => s.StartDate().getMonth());
            } else {
                this.courseStreams.set(this.course.Recordings, (s: Stream) =>
                    this.getTrueWeek(s.GetWeekNumber(this.dateOfFirstWeek)),
                );
            }

            // update group names
            const groups = this.courseStreams.get(
                this.sortFn(this.streamSortMode),
                this.filterPred(this.streamFilterMode),
            );
            this.groupNames.clear();
            for (let i = 0; i < groups.length; i++) {
                const s1 = groups[i][0];
                this.groupNames.set(s1.ID, this.getGroupName(s1));
                const s2 = groups[i][groups[i].length - 1];
                this.groupNames.set(s2.ID, this.getGroupName(s2));
            }
        },

        /**
         * Maps the difference in Weeks between any lecture and the first lecture to the true week count ignoring weeks without lectures (e.g. Christmas Break)
         */
        initializeWeekMap() {
            let latestWeek = 1;
            this.course.Recordings.sort(this.sortFn(StreamSortMode.OldestFirst)).forEach((s: Stream, i: number) => {
                if (i === 0) {
                    this.dateOfFirstWeek = s.StartDate();
                    this.dateOfFirstWeek = new Date(
                        this.dateOfFirstWeek.getTime() - this.dateOfFirstWeek.getDay() * 1000 * 60 * 60 * 24,
                    );
                    this.dateOfFirstWeek.setHours(0, 1); // avoids errors e.g. in case week1 has vod on Monday at 10am, week2 at 8am
                }
                const week = s.GetWeekNumber(this.dateOfFirstWeek);
                if (!this.weekCountWithoutEmptyWeeks.has(week)) {
                    this.weekCountWithoutEmptyWeeks.set(week, latestWeek++);
                }
            });
        },

        getTrueWeek(n: number): number {
            return this.weekCountWithoutEmptyWeeks.get(n);
        },

        getGroupName(s: Stream): string {
            if (this.groupMode === GroupMode.Month) {
                return s.GetMonthName();
            } else {
                return "Week " + this.getTrueWeek(s.GetWeekNumber(this.dateOfFirstWeek)).toString();
            }
        },

        /**
         * Depending on the pinned value, pin or unpin course
         */
        pin() {
            if (this.course.Pinned) {
                UserAPI.unpinCourse(this.course.ID);
            } else {
                UserAPI.pinCourse(this.course.ID);
            }
            this.course.Pinned = !this.course.Pinned;
            Tunnel.pinned.add({ pin: this.course.Pinned, course: this.course });
        },

        copyHLS(stream: Stream, dropdown: ToggleableElement) {
            copyToClipboard(stream.HLSUrl);
            dropdown.toggle(false);
        },

        async loadCourse() {
            this.course = await CoursesAPI.get(this.slug, this.year, this.term, this.userId);
        },

        async loadPinned() {
            this.course.Pinned = ((await UserAPI.hasPinnedCourse(this.course.ID)) as HasPinnedCourseDTO).has;
        },

        async loadProgresses(ids: number[]) {
            if (ids.length > 0) {
                return ProgressAPI.getBatch(ids);
            }
        },
    } as AlpineComponent;
}
