import { Course, CoursesAPI, Stream } from "../api/courses";
import { ProgressAPI } from "../api/progress";
import { Paginator } from "../utilities/paginator";
import { HasPinnedCourseDTO, UserAPI } from "../api/user";
import { copyToClipboard } from "../utilities/input-interactions";

export enum StreamSortMode {
    NewestFirst,
    OldestFirst,
}

export enum StreamFilterMode {
    ShowWatched,
    HideWatched,
}

export function courseContext(slug: string, year: number, term: string) {
    return {
        slug: slug as string,
        year: year as number,
        term: term as string,

        course: {} as Course,

        courseStreams: new Paginator<Stream>([], 8),

        plannedStreams: [],

        streamSortMode: StreamSortMode.NewestFirst,
        streamFilterMode: StreamFilterMode.ShowWatched,

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
                .catch((_) => {
                    document.location.href = `/new?year=${year}&term=${term}`; // redirect to start page on error
                })
                .then(() => {
                    this.loadPinned();
                    this.plannedStreams = this.groupBy(this.course.Planned, (s) => s.MonthOfStart());
                    this.loadProgresses(this.course.Recordings.map((s: Stream) => s.ID)).then((progresses) => {
                        this.courseStreams.set(this.course.Recordings);
                        this.courseStreams.forEach((s: Stream, i) => (s.Progress = progresses[i]));
                        this.courseStreams.reset();
                    });
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
        },

        isNewestFirst(): boolean {
            return this.streamSortMode === StreamSortMode.NewestFirst;
        },

        sortOldestFirst() {
            this.streamSortMode = StreamSortMode.OldestFirst;
        },

        isOldestFirst(): boolean {
            return this.streamSortMode === StreamSortMode.OldestFirst;
        },

        toggleShowWatched() {
            this.streamFilterMode =
                this.streamFilterMode === StreamFilterMode.ShowWatched
                    ? StreamFilterMode.HideWatched
                    : StreamFilterMode.ShowWatched;
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
        },

        copyHLS(stream: Stream) {
            copyToClipboard(stream.HLSUrl);
        },

        async loadCourse() {
            this.course = await CoursesAPI.get(this.slug, this.year, this.term);
        },

        async loadPinned() {
            const pinned = (await UserAPI.hasPinnedCourse(this.course.ID)) as HasPinnedCourseDTO;
            this.course.Pinned = pinned.has;
        },

        async loadProgresses(ids: number[]) {
            if (ids.length > 0) {
                return ProgressAPI.getBatch(ids);
            }
        },

        groupBy<K extends keyof any>(arr: Stream[], key: (s: Stream) => K) {
            return arr.reduce((groups, item) => {
                (groups[key(item)] ||= []).push(item);
                return groups;
            }, {} as Record<K, Stream[]>);
        },
    };
}
