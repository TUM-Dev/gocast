import { Course, CoursesAPI, Stream } from "../api/courses";
import { ProgressAPI } from "../api/progress";
import { Paginator } from "../utilities/paginator";

export enum StreamSortMode {
    NewestFirst,
    OldestFirst,
}

export function courseContext(initial: Course) {
    return {
        course: {},
        courseStreams: new Paginator<Stream>([], 8),
        streamSortMode: StreamSortMode.NewestFirst,

        init() {
            this.reset(initial);
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

        async reset(c: Course) {
            const progresses = await this.loadProgresses(c.Recordings.map((s) => s.ID));
            c.Recordings.forEach((s, i) => (s.Progress = progresses[i]));
            // TODO: Endless loop
            this.course = c;

            this.courseStreams.set(this.course.Recordings);
            this.courseStreams.reset();
        },

        async loadProgresses(ids: number[]) {
            if (ids.length > 0) {
                return ProgressAPI.getBatch(ids);
            }
        },
    };
}
