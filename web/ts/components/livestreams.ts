import { CoursesAPI, Livestream } from "../api/courses";
import { AlpineComponent } from "./alpine-component";

export function livestreams(predicate?: (s: Livestream) => boolean): AlpineComponent {
    return {
        _all: [] as Livestream[],

        livestreams: [] as Livestream[],
        init() {
            this.reload();
        },

        async reload() {
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
    } as AlpineComponent;
}
