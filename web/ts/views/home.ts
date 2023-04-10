import { ToggleableElement } from "../utilities/ToggleableElement";
import { Semester, SemesterDTO, SemestersAPI } from "../api/semesters";
import { ProgressAPI } from "../api/progress";
import { Course, CoursesAPI, Livestream } from "../api/courses";

export enum Views {
    Main,
    UserCourses,
    PublicCourses,
}

type History = {
    year: number;
    term: string;
    view: Views;
};

export function context() {
    const url = new URL(window.location.href);
    return {
        term: url.searchParams.get("term") ?? undefined,
        year: +url.searchParams.get("year"),
        view: +url.searchParams.get("view") ?? Views.Main,

        semesters: [] as Semester[],
        currentSemesterIndex: -1,
        selectedSemesterIndex: -1,

        livestreams: [] as Livestream[],
        publicCourses: [] as Course[],
        userCourses: [] as Course[],
        liveToday: [] as Course[],
        recently: [] as Course[],

        navigation: new ToggleableElement(new Map([["allSemesters", new ToggleableElement()]])),

        loadingIndicator: 0,
        nothingToDo: false,

        /**
         * AlpineJS init function which is called automatically in addition to 'x-init'
         */
        init() {
            this.reload(true);
        },

        /**
         * Load context
         * @param  {boolean} full If true, load everything including semesters and livestreams
         */
        reload(full = false) {
            const promises = full
                ? [this.loadSemesters(), this.loadPublicCourses(), this.loadLivestreams(), this.loadUserCourses()]
                : [this.loadPublicCourses(), this.loadUserCourses()];
            this.load(promises).then(() => {
                this.nothingToDo =
                    this.livestreams.length === 0 && this.liveToday.length === 0 && this.recently.length === 0;
            });
            promises[promises.length - 1].then(() => {
                this.recently = this.getRecently();
                this.liveToday = this.getLiveToday();
                this.loadProgresses(this.userCourses.map((c) => c.LastLecture.ID));
            });
        },

        /**
         * Resolve given promises and increment loadingIndicator partially
         * @param  {Promise<object>[]} promises Array of promises
         */
        load(promises: Promise<object>[]): Promise<number | void> {
            this.loadingIndicator = 0;
            promises.forEach((p) => {
                Promise.resolve(p).then((_) => (this.loadingIndicator += 100 / promises.length));
            });
            return Promise.all(promises).then(() => (this.loadingIndicator = 0));
        },

        /**
         * Event triggered by clicking any browser's back or forward buttons
         */
        onPopState(event: PopStateEvent) {
            const state = event.state || {};
            const year = +state["year"] || this.semesters[this.currentSemesterIndex].Year;
            const term = state["term"] || this.semesters[this.currentSemesterIndex].TeachingTerm;
            this.view = state["view"] || Views.Main;
            this.switchSemester(year, term, false);
        },

        showMain() {
            this.switchView(Views.Main);
            this.pushHistory(this.year, this.term, Views.Main);
        },

        showUserCourses() {
            this.switchView(Views.UserCourses);
            this.pushHistory(this.year, this.term, Views.UserCourses);
        },

        showPublicCourses() {
            this.switchView(Views.PublicCourses);
            this.pushHistory(this.year, this.term, Views.PublicCourses);
        },

        switchView(view: Views) {
            this.view = view;
            this.navigation.toggle(false);
        },

        async loadSemesters() {
            const res: SemesterDTO = await SemestersAPI.get();
            this.semesters = res.Semesters;

            this.currentSemesterIndex = this.findSemesterIndex(res.Current.Year, res.Current.TeachingTerm);
            if (this.year !== null && this.term != null) {
                this.selectedSemesterIndex = this.findSemesterIndex(this.year, this.term);
            }

            if (this.selectedSemesterIndex === -1) {
                this.selectedSemesterIndex = this.currentSemesterIndex;
                this.year = res.Current.Year;
                this.term = res.Current.TeachingTerm;
            }
        },

        async loadLivestreams() {
            this.livestreams = await CoursesAPI.getLivestreams();
        },

        async loadPublicCourses() {
            this.publicCourses = await CoursesAPI.getPublic(this.year, this.term);
        },

        async loadUserCourses() {
            this.userCourses = await CoursesAPI.getUsers(this.year, this.term);
        },

        async loadProgresses(ids: number[]) {
            if (ids.length > 0) {
                const progresses = await ProgressAPI.getBatch(ids);
                this.recently.forEach((r, i) => (r.LastLecture.Progress = progresses[i]));
            }
        },

        /**
         * Filter userCourses for lectures streamed today
         */
        getLiveToday() {
            const today = new Date();
            const eq = (a: Date, b: Date) =>
                a.getDay() === b.getDay() && a.getMonth() == b.getMonth() && a.getFullYear() === b.getFullYear();
            return this.userCourses
                .filter((c) => c.NextLecture.ID !== 0)
                .filter((c) => eq(today, new Date(c.NextLecture.Start)));
        },

        /**
         * Filter userCourses for recently streamed lectures
         */
        getRecently() {
            return this.userCourses.filter((c) => c.LastLecture.ID !== 0);
        },

        /**
         * Switch context to a different semester
         * @param  {string} year The year to switch to
         * @param  {object} term The teaching term to switch to
         * @param  {object} pushState Push new state into the browser's history?
         */
        async switchSemester(year: number, term: string, pushState = true) {
            this.year = year;
            this.term = term;
            this.selectedSemesterIndex = this.findSemesterIndex(this.year, this.term);
            this.navigation.getChild("allSemesters").toggle(false);

            if (pushState) {
                this.pushHistory(year, term);
            }

            this.reload();
        },

        /**
         * Return index of the given year and term values in the semesters array or -1 if not found.
         */
        findSemesterIndex(year: number, term: string) {
            return this.semesters.findIndex((s) => s.Year === year && s.TeachingTerm === term);
        },

        /**
         * Update search parameters and push state into the browser's history
         */
        pushHistory(year: number, term: string, view?: Views) {
            url.searchParams.set("year", String(year));
            url.searchParams.set("term", term);
            let data = { year, term } as History;
            if (view !== undefined) {
                if (view !== Views.Main) {
                    url.searchParams.set("view", view.toString());
                    data = { ...data, view };
                } else {
                    url.searchParams.delete("view");
                }
            }
            window.history.pushState(data, "", url.toString());
        },
    };
}
