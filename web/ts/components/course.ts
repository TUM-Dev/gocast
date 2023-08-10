import { Course, CoursesAPI, Stream } from "../api/courses";
import { ProgressAPI } from "../api/progress";
import { Paginator } from "../utilities/paginator";
import { HasPinnedCourseDTO, UserAPI } from "../api/user";
import { copyToClipboard } from "../utilities/input-interactions";
import { AlpineComponent } from "./alpine-component";
import { Tunnel } from "../utilities/tunnels";
import { ToggleableElement } from "../utilities/ToggleableElement";
import { getFromStorage, setInStorage } from "../utilities/storage";

export enum StreamSortMode {
    NewestFirst,
    OldestFirst,
}

export enum StreamFilterMode {
    ShowWatched,
    HideWatched,
}

export function courseContext(slug: string, year: number, term: string): AlpineComponent {
    return {
        slug: slug as string,
        year: year as number,
        term: term as string,

        course: new Course() as Course,

        courseStreams: new Paginator<Stream>([], 8, (s: Stream) => s.FetchThumbnail()),
        plannedStreams: new Paginator<Stream>([], 3),
        upcomingStreams: new Paginator<Stream>([], 3),

        streamSortMode: +getFromStorage("streamSortMode") ?? StreamSortMode.NewestFirst,
        streamFilterMode: +getFromStorage("streamFilterMode") ?? StreamFilterMode.ShowWatched,

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
                    this.loadProgresses(this.course.Recordings.map((s: Stream) => s.ID)).then((progresses) => {
                        this.courseStreams
                            .set(this.course.Recordings)
                            .forEach((s: Stream, i) => (s.Progress = progresses[i]))
                            .reset()
                            .preload(this.sortFn(this.streamSortMode));
                    });
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
            this.course = await CoursesAPI.get(this.slug, this.year, this.term);
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
