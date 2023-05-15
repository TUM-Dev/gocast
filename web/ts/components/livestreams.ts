import { CoursesAPI, Livestream } from "../api/courses";

export function livestreams(predicate?: (s: Livestream) => boolean) {
    return {
        _all: [] as Livestream[],

        livestreams: [] as Livestream[],
        init() {
            this.load();
        },

        async load() {
            this._all = await CoursesAPI.getLivestreams();
            this.livestreams = predicate ? this._all.filter(predicate) : this._all;
            console.log("🌑 init livestreams", this.livestreams);
        },

        refilter(predicate?: (s: Livestream) => boolean) {
            this.livestreams = predicate ? this._all.filter(predicate) : this._all;
        },

        hasLivestreams() {
            return this.livestreams.length > 0;
        },
    };
}
