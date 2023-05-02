import { Course, CoursesAPI, Stream } from "../api/courses";
import { ProgressAPI } from "../api/progress";
import { Paginator } from "../utilities/paginator";

export enum StreamSortMode {
    NewestFirst,
    OldestFirst,
}

export function course(year: number, term: string, slug: string) {
    return {
        course: {} as Course,
        courseStreams: new Paginator<Stream>([], 8),
        streamSortMode: StreamSortMode.NewestFirst,

        init() {
            this.reset();
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

        async reset() {
            this.course = await CoursesAPI.get(year, term, slug);
            console.log(this.course);
            this.courseStreams.set(this.course.Recordings);
            this.courseStreams.reset();

            await this.loadProgresses(this.course.Recordings.map((s) => s.ID));
        },

        async loadProgresses(ids: number[]) {
            if (ids.length > 0) {
                const progresses = await ProgressAPI.getBatch(ids);
                this.course.Recordings.forEach((s, i) => (s.Progress = progresses[i]));
            }
        },
    };
}
