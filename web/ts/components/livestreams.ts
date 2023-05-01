import { CoursesAPI, Livestream } from "../api/courses";

export function livestreams(predicate?: (s: Livestream) => boolean) {
    return {
        livestreams: [] as Livestream[],
        init() {
            this.load();
        },

        async load() {
            const s = await CoursesAPI.getLivestreams();
            this.livestreams = predicate ? s.filter(predicate) : s;
            console.log("init livestreams", this.livestreams);
        },

        hasLivestreams() {
            return this.livestreams.length > 0;
        },
    };
}
