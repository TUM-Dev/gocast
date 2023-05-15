import { ToggleableElement } from "../utilities/ToggleableElement";
import { Semester, SemesterDTO, SemestersAPI } from "../api/semesters";
import { ProgressAPI } from "../api/progress";
import { Course, CoursesAPI } from "../api/courses";
import { Paginator } from "../utilities/paginator";
import { courseContext } from "../components/course";
import { date_eq } from "../utilities/time-utils";

export enum Views {
    Main,
    UserCourses,
    PublicCourses,
    Course,
}

type History = {
    year: number;
    term: string;
    slug: string;
    view: Views;
};

export function skeleton() {
    const url = new URL(window.location.href);
    return {
        state: {
            term: url.searchParams.get("term") ?? undefined,
            year: +url.searchParams.get("year"),
            slug: url.searchParams.get("slug") ?? undefined,
            view: +url.searchParams.get("view") ?? Views.Main,

            isMain: function () {
                return this.view === Views.Main;
            },
            isCourse: function () {
                return this.view === Views.Course;
            },
            isPublicCourses: function () {
                return this.view === Views.PublicCourses;
            },
            isUserCourses: function () {
                return this.view === Views.UserCourses;
            },
        },

        semesters: [] as Semester[],
        currentSemesterIndex: -1,
        selectedSemesterIndex: -1,

        publicCourses: [] as Course[],
        userCourses: [] as Course[],
        pinnedCourses: [] as Course[],

        navigation: new ToggleableElement([["allSemesters", new ToggleableElement()]]),

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
            Promise.all(
                [
                    full ? [this.loadSemesters()] : [],
                    this.loadPublicCourses(),
                    this.loadPinnedCourses(),
                    this.loadUserCourses(),
                ].flat(),
            );
        },

        /**
         * Event triggered by clicking any browser's back or forward buttons
         */
        onPopState(event: PopStateEvent) {
            const state = event.state || {};
            const year = +state["year"] || this.semesters[this.currentSemesterIndex].Year;
            const term = state["term"] || this.semesters[this.currentSemesterIndex].TeachingTerm;
            this.state.view = state["view"] || Views.Main;
            this.state.slug = state["slug"] || undefined;
            this.switchSemester(year, term, false);
        },

        showMain() {
            this.switchView(Views.Main);
            this.pushHistory({ year: this.state.year, term: this.state.term, view: Views.Main });
        },

        showUserCourses() {
            this.switchView(Views.UserCourses);
            this.pushHistory({ year: this.state.year, term: this.state.term, view: Views.UserCourses });
        },

        showPublicCourses() {
            this.switchView(Views.PublicCourses);
            this.pushHistory({ year: this.state.year, term: this.state.term, view: Views.PublicCourses });
        },

        showCourse(slug: string) {
            this.state.slug = slug;
            this.switchView(Views.Course);
            this.pushHistory({
                year: this.state.year,
                term: this.state.term,
                view: Views.Course,
                slug: this.state.slug,
            });
        },

        switchView(view: Views) {
            this.state.view = view;
            this.navigation.toggle(false);
        },

        async loadSemesters() {
            const res: SemesterDTO = await SemestersAPI.get();
            this.semesters = res.Semesters;

            this.currentSemesterIndex = this.findSemesterIndex(res.Current.Year, res.Current.TeachingTerm);
            if (this.state.year !== null && this.state.term != null) {
                this.selectedSemesterIndex = this.findSemesterIndex(this.state.year, this.state.term);
            }

            if (this.selectedSemesterIndex === -1) {
                this.selectedSemesterIndex = this.currentSemesterIndex;
                this.state.year = res.Current.Year;
                this.state.term = res.Current.TeachingTerm;
            }
        },

        async loadPublicCourses() {
            this.publicCourses = await CoursesAPI.getPublic(this.state.year, this.state.term);
        },

        async loadUserCourses() {
            this.userCourses = await CoursesAPI.getUsers(this.state.year, this.state.term);
        },

        async loadPinnedCourses() {
            this.pinnedCourses = await CoursesAPI.getPinned(this.state.year, this.state.term);
        },

        /**
         * Switch context to a different semester
         * @param  {string} year The year to switch to
         * @param  {object} term The teaching term to switch to
         * @param  {object} pushState Push new state into the browser's history?
         */
        async switchSemester(year: number, term: string, pushState = true) {
            this.state.year = year;
            this.state.term = term;
            this.selectedSemesterIndex = this.findSemesterIndex(this.state.year, this.state.term);
            this.navigation.getChild("allSemesters").toggle(false);

            if (pushState) {
                this.pushHistory({ year, term });
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
        pushHistory(history: { year: number; term: string; slug?: string; view?: Views }) {
            url.searchParams.set("year", String(history.year));
            url.searchParams.set("term", history.term);

            let data = { year: history.year, term: history.term } as History;

            if (history.slug !== undefined) {
                url.searchParams.set("slug", history.slug);
                data = { ...data, slug: history.slug };
            }

            if (history.view !== undefined) {
                if (history.view !== Views.Main) {
                    url.searchParams.set("view", history.view.toString());
                    data = { ...data, view: history.view };
                } else {
                    url.searchParams.delete("view");
                    url.searchParams.delete("slug");
                }
            }
            window.history.pushState(data, "", url.toString());
        },
    };
}
