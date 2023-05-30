import { date_eq } from "../utilities/time-utils";
import { Course, CoursesAPI } from "../api/courses";
import { Paginator } from "../utilities/paginator";
import { ProgressAPI } from "../api/progress";

export function mainContext(year: number, term: string) {
    return {
        year: year as number,
        term: term as string,

        userCourses: [] as Course[],
        liveToday: [] as Course[],
        recently: new Paginator<Course>([], 10, Course.LoadThumbnail),

        /**
         * AlpineJS init function which is called automatically in addition to 'x-init'
         */
        init() {
            this.reload(this.slug, this.year, this.term);
        },

        /**
         * (Re-)Load course context
         */
        reload(year: number, term: string) {
            this.year = year;
            this.term = term;
            this.loadUserCourses()
                .catch((err) => {
                    console.error(err);
                })
                .then(() => {
                    this.recently.set(this.getRecently()).reset();
                    this.liveToday = this.getLiveToday();
                    this.loadProgresses(this.userCourses.map((c) => c.LastRecording.ID));
                    console.log("🌑 init recently", this.recently);
                    console.log("🌑 init live today", this.liveToday);
                });
        },

        /**
         * Filter userCourses for lectures streamed today
         */
        getLiveToday() {
            const today = new Date();
            return this.userCourses
                .filter((c) => c.NextLecture.ID !== 0)
                .filter((c) => date_eq(today, new Date(c.NextLecture.Start)));
        },

        /**
         * Filter userCourses for recently streamed lectures
         */
        getRecently() {
            return this.userCourses.filter((c) => c.LastRecording.ID !== 0);
        },

        async loadUserCourses() {
            this.userCourses = await CoursesAPI.getUsers(this.state.year, this.state.term);
        },

        async loadProgresses(ids: number[]) {
            if (ids.length > 0) {
                const progresses = await ProgressAPI.getBatch(ids);
                this.recently.forEach((r, i) => (r.LastRecording.Progress = progresses[i]));
            }
        },
    };
}
