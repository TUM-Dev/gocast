import { ToggleableElement } from "../utilities/ToggleableElement";
import { Semester, SemesterDTO, SemestersAPI } from "../api/semesters";
import { Course, CoursesAPI } from "../api/courses";
import { AlpineComponent } from "../components/alpine-component";
import { PinnedUpdate, Tunnel } from "../utilities/tunnels";

export function skeleton(): AlpineComponent {
    return {
        state: new PageState(new URL(window.location.href)),

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
            const callback = (update) => this.updatePinned(update);
            Tunnel.pinned.subscribe(callback);
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
            this.state.update({ view: state["view"] || View.Main, slug: state["slug"] });
            this.switchSemester(year, term, false);
        },

        showMain() {
            this.switchView(View.Main);
            this.state.pushHistory({ year: this.state.year, term: this.state.term, view: View.Main });
        },

        showUserCourses() {
            this.switchView(View.UserCourses);
            this.state.pushHistory({ year: this.state.year, term: this.state.term, view: View.UserCourses });
        },

        showPublicCourses() {
            this.switchView(View.PublicCourses);
            this.state.pushHistory({ year: this.state.year, term: this.state.term, view: View.PublicCourses });
        },

        showCourse(slug: string, year?: number, term?: string) {
            this.state.update({ slug, year, term });
            this.switchView(View.Course);
            this.state.pushHistory({
                year: this.state.year,
                term: this.state.term,
                view: View.Course,
                slug: this.state.slug,
            });
        },

        switchView(view: View) {
            this.state.update({ view });
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
                this.state.update({ year: res.Current.Year, term: res.Current.TeachingTerm });
            }
        },

        async loadPublicCourses() {
            this.publicCourses = (await CoursesAPI.getPublic(this.state.year, this.state.term)).sort(Course.Compare);
        },

        async loadUserCourses() {
            this.userCourses = (await CoursesAPI.getUsers(this.state.year, this.state.term)).sort(Course.Compare);
        },

        async loadPinnedCourses() {
            this.pinnedCourses = (await CoursesAPI.getPinned(this.state.year, this.state.term)).sort(Course.Compare);
        },

        /**
         * Switch context to a different semester
         * @param  {string} year The year to switch to
         * @param  {object} term The teaching term to switch to
         * @param  {object} pushState Push new state into the browser's history?
         */
        async switchSemester(year: number, term: string, pushState = true) {
            this.state.update({ year, term });
            this.selectedSemesterIndex = this.findSemesterIndex(this.state.year, this.state.term);
            this.navigation.getChild("allSemesters").toggle(false);

            if (pushState) {
                this.state.pushHistory({ year, term });
            }

            this.reload();
        },

        /**
         * Return index of the given year and term values in the semesters array or -1 if not found.
         */
        findSemesterIndex(year: number, term: string) {
            return this.semesters.findIndex((s) => s.Year === year && s.TeachingTerm === term);
        },

        updatePinned(update: PinnedUpdate) {
            if (update.pin) {
                this.pinnedCourses = [...this.pinnedCourses, update.course].sort(Course.Compare);
            } else {
                this.pinnedCourses = this.pinnedCourses.filter((c) => c.ID !== update.course.ID);
            }
        },
    } as AlpineComponent;
}

enum View {
    Main,
    UserCourses,
    PublicCourses,
    Course,
}

type History = {
    year: number;
    term: string;
    slug: string;
    view: View;
};

class PageState {
    private readonly url: URL;
    term: string;
    year: number;
    slug: string;
    view: View | string;

    constructor(url: URL) {
        this.url = url;
        this.term = url.searchParams.get("term") ?? undefined;
        this.year = +url.searchParams.get("year") ?? undefined;
        if (this.year === 0) {
            this.year = undefined;
        }
        this.slug = url.searchParams.get("slug") ?? undefined;
        this.view = url.searchParams.get("view") ?? View.Main;
    }

    update(state: { term?: string; year?: number; slug?: string; view?: View }) {
        this.term = state.term ?? this.term;
        this.year = state.year ?? this.year;
        this.slug = state.slug ?? this.slug;
        this.view = state.view ?? this.view;
    }

    isMain() {
        return this.view == View.Main;
    }

    isCourse() {
        return this.view == View.Course;
    }

    isPublicCourses() {
        return this.view == View.PublicCourses;
    }

    isUserCourses() {
        return this.view == View.UserCourses;
    }

    /**
     * Update search parameters and push state into the browser's history
     */
    pushHistory(history: { year: number; term: string; slug?: string; view?: View }) {
        this.url.searchParams.set("year", String(history.year));
        this.url.searchParams.set("term", history.term);

        let data = { year: history.year, term: history.term } as History;

        if (history.slug !== undefined) {
            this.url.searchParams.set("slug", history.slug);
            data = { ...data, slug: history.slug };
        }

        if (history.view !== undefined) {
            if (history.view !== View.Main) {
                this.url.searchParams.set("view", history.view.toString());
                data = { ...data, view: history.view };
            } else {
                this.url.searchParams.delete("view");
                this.url.searchParams.delete("slug");
            }
        }
        window.history.pushState(data, "", this.url.toString());
    }
}
