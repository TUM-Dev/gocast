import { Course, CoursesAPI, Stream } from "../api/courses";
import { ProgressAPI } from "../api/progress";
import { Paginator } from "../utilities/paginator";
import { HasPinnedCourseDTO, UserAPI } from "../api/user";

export enum StreamSortMode {
    NewestFirst,
    OldestFirst,
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

        /**
         * AlpineJS init function which is called automatically in addition to 'x-init'
         */
        init() {
            this.load();
        },

        /**
         * (Re-)Load course context
         */
        reload(slug: string, year: number, term: string) {
            this.slug = slug;
            this.year = year;
            this.term = term;
            this.load();
        },

        compareStream(sortMode: StreamSortMode) {
            return sortMode === StreamSortMode.NewestFirst
                ? (a: Stream, b: Stream) => a.CompareStart(b)
                : (a: Stream, b: Stream) => a.CompareStart(b) * -1;
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

        pin() {
            if (this.course.Pinned) {
                UserAPI.unpinCourse(this.course.ID);
            } else {
                UserAPI.pinCourse(this.course.ID);
            }
            this.course.Pinned = !this.course.Pinned;
        },

        async load() {
            this.course = await CoursesAPI.get(this.slug, this.year, this.term);

            this.loadPinned();

            this.plannedStreams = this.groupBy(this.course.Planned, (s) => s.MonthOfStart());

            const progresses = await this.loadProgresses(this.course.Recordings.map((s: Stream) => s.ID));

            this.courseStreams.set(this.course.Recordings);
            this.courseStreams.forEach((s: Stream, i) => (s.Progress = progresses[i]));
            this.courseStreams.reset();
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
