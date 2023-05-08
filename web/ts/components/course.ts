import { Course, CoursesAPI, Stream } from "../api/courses";
import { ProgressAPI } from "../api/progress";
import { Paginator } from "../utilities/paginator";

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
            this.course.Pinned = !this.course.Pinned;
        },

        async load() {
            this.course = await CoursesAPI.get(this.slug, this.year, this.term);
            const progresses = await this.loadProgresses(this.course.Recordings.map((s: Stream) => s.ID));

            this.courseStreams.set(this.course.Recordings);
            this.courseStreams.forEach((s: Stream, i) => (s.Progress = progresses[i]));
            this.courseStreams.reset();
        },

        async loadProgresses(ids: number[]) {
            if (ids.length > 0) {
                return ProgressAPI.getBatch(ids);
            }
        },
    };
}
