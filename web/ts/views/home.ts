export function header() {
    return {
        userContext: new ToggleableElement(new Map([["themePicker", new ToggleableElement()]])),

        notifications: new Notifications(),
        notification: new ToggleableElement(),
        toggleNotification(set?: boolean) {
            this.notification.toggle(set);
            this.notifications.writeToStorage(true);
        },
    };
}

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

const DEFAULT_LECTURE_NAME = "Untitled lecture";

export function context() {
    const url = new URL(window.location.href);
    return {
        term: url.searchParams.get("term") ?? undefined,
        year: +url.searchParams.get("year"),
        view: +url.searchParams.get("view") ?? Views.Main,

        navigation: new ToggleableElement(new Map([["allSemesters", new ToggleableElement()]])),

        semesters: [] as SemesterItem[],
        currentSemesterIndex: -1,
        selectedSemesterIndex: -1,

        livestreams: [],
        publicCourses: [],
        userCourses: [],
        liveToday: [],
        recently: [],

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
        load(promises: Promise<object>[]): Promise<any> {
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
            const res: SemesterResponse = await Semesters.get();
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
            this.livestreams = await Courses.getLivestreams();
        },

        async loadPublicCourses() {
            this.publicCourses = await Courses.getPublic(this.year, this.term);
        },

        async loadUserCourses() {
            this.userCourses = await Courses.getUsers(this.year, this.term);
        },

        async loadProgresses(ids: number[]) {
            if (ids.length > 0) {
                const progresses = await Progress.getBatch(ids);
                this.recently.forEach((r, i) => (r.LastLecture.Progress = progresses[i]));
            }
        },

        /**
         * Filter userCourses for lectures streamed today
         */
        getLiveToday() {
            return this.userCourses.filter((c) => {
                if (c.NextLecture.ID !== 0) {
                    const start = new Date(c.NextLecture.Start);
                    const now = new Date();
                    return (
                        start.getDay() === now.getDay() &&
                        start.getMonth() == now.getMonth() &&
                        start.getFullYear() === now.getFullYear()
                    );
                }

                return false;
            });
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
            this.allSemesters.toggle(false);

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

class ToggleableElement {
    private readonly children: Map<string, ToggleableElement>;

    public value: boolean;

    constructor(children?: Map<string, ToggleableElement>, value = false) {
        this.children = children || new Map<string, ToggleableElement>();
        this.value = value;
    }

    getChild(name: string): ToggleableElement {
        return this.children.get(name);
    }

    toggle(set?: boolean) {
        this.value = set ?? !this.value;
        if (!this.value) {
            this.children.forEach((c) => c.toggle(false));
        }
    }
}

class Notifications {
    notifications: Notification[] = [];

    constructor() {
        this.notifications = [];
    }

    getAll(): Notification[] {
        return this.notifications;
    }

    empty(): boolean {
        return this.notifications.length === 0;
    }

    writeToStorage(markRead = false) {
        if (markRead) {
            this.notifications.forEach((notification) => {
                notification.read = true;
            });
        }
        localStorage.setItem("notifications", JSON.stringify(this.notifications));
    }

    hasNewNotifications(): boolean {
        return this.notifications.some((notification) => !notification.read);
    }

    fetchNotifications(): void {
        this.notifications = JSON.parse(localStorage.getItem("notifications") || "[]");

        const lastNotificationFetch: Date = new Date(parseInt(localStorage.getItem("lastNotificationFetch") || "0"));
        // fetch every 10 minutes at most:
        if (new Date().getTime() - lastNotificationFetch.getTime() > 1000 * 60 * 10) {
            fetch(`/api/notifications/`)
                .then((response) => response.json() as Promise<Notification[]>)
                .then((data) => {
                    // merge new notifications read status with existing ones:
                    for (let i = 0; i < this.notifications.length; i++) {
                        for (let j = 0; j < data.length; j++) {
                            if (data[j].id === this.notifications[i].id) {
                                data[j].read = this.notifications[i].read;
                                break;
                            }
                        }
                    }
                    this.notifications = data;
                    this.writeToStorage();
                    localStorage.setItem("lastNotificationFetch", new Date().getTime().toString());
                });
        }
    }
}

export class Notification {
    id: number;
    createdAt: Date;
    title: string | undefined;
    body: string;
    read: boolean;
    target: number;

    constructor(title: string | undefined, body: string, target: number) {
        this.title = title;
        this.body = body;
        this.target = target;
    }
}

type SemesterResponse = {
    Current: SemesterItem;
    Semesters: SemesterItem[];
};

class SemesterItem {
    TeachingTerm: string;
    Year: number;

    constructor(obj: SemesterItem) {
        this.TeachingTerm = obj.TeachingTerm;
        this.Year = obj.Year;
    }

    public FriendlyString(): string {
        return `${this.TeachingTerm === "W" ? "Winter" : "Summer"} ${this.Year}`;
    }
}

/**
 * REST API Wrapper for /api/semesters
 */
const Semesters = {
    async get(): Promise<SemesterResponse> {
        return get("/api/semesters").then((l: SemesterResponse) => {
            l.Semesters = l.Semesters.map((s) => new SemesterItem(s));
            return l;
        });
    },
};

class Stream {
    readonly ID: number;
    readonly Name: string;
    readonly End: string;
    readonly Start: string;

    Progress?: ProgressItem;

    static New(obj): Stream {
        const s = Object.assign(new Stream(), obj);
        s.Name = s.Name === "" ? DEFAULT_LECTURE_NAME : s.Name;
        return s;
    }

    public FriendlyDateStart(): string {
        return new Date(this.Start).toLocaleString();
    }

    public UntilString(): string {
        const end = new Date(this.End);
        const hours = end.getHours();
        const minutes = end.getMinutes();
        return `Until ${hours}:${minutes < 10 ? minutes + "0" : minutes}`;
    }
}

class Course {
    readonly ID: number;
    readonly Visibility: string;
    readonly Slug: string;
    readonly Year: number;
    readonly TeachingTerm: string;
    readonly Name: string;

    readonly NextLecture?: Stream;
    readonly LastLecture?: Stream;

    static New(obj): Course {
        const c = Object.assign(new Course(), obj);
        c.NextLecture = c.NextLecture ? Stream.New(obj.NextLecture) : undefined;
        c.LastLecture = c.LastLecture ? Stream.New(obj.LastLecture) : undefined;
        return c;
    }

    public URL(): string {
        return `/course/${this.Year}/${this.TeachingTerm}/${this.Slug}`;
    }

    public LastLectureURL(): string {
        return `/w/${this.Slug}/${this.LastLecture.ID}`;
    }

    public NextLectureURL(): string {
        return `/w/${this.Slug}/${this.NextLecture.ID}`;
    }

    public IsHidden(): boolean {
        return this.Visibility === "hidden";
    }
}

class LectureHall {
    readonly Name: string;

    constructor(obj: LectureHall) {
        this.Name = obj.Name;
    }
}

class Livestream {
    readonly Stream: Stream;
    readonly Course: Course;
    readonly LectureHall?: LectureHall;

    constructor(obj: Livestream) {
        this.Stream = Stream.New(obj.Stream);
        this.Course = Course.New(obj.Course);
        this.LectureHall = obj.LectureHall ? new LectureHall(obj.LectureHall) : undefined;
    }

    public InLectureHall(): boolean {
        return this.LectureHall !== undefined;
    }
}

/**
 * REST API Wrapper for /api/courses
 */
const Courses = {
    async getLivestreams() {
        return get("/api/courses/live").then((livestreams) => livestreams.map((l) => new Livestream(l)));
    },

    async getPublic(year?: number, term?: string): Promise<object> {
        return get(`/api/courses/public${this.query(year, term)}`).then((courses) => courses.map((c) => Course.New(c)));
    },

    async getUsers(year?: number, term?: string): Promise<object> {
        return get(`/api/courses/users${this.query(year, term)}`).then((courses) => courses.map((c) => Course.New(c)));
    },

    query: (year?: number, term?: number) =>
        year !== undefined && term !== undefined && year !== 0 ? `?year=${year}&term=${term}` : "",
};

class ProgressItem {
    private readonly progress: number;
    private readonly watched: boolean;
    private readonly streamId: number;

    constructor(obj: ProgressItem) {
        this.progress = obj.progress;
        this.watched = obj.watched;
        this.streamId = obj.streamId;
    }

    public Percentage(): number {
        return Math.round(this.progress * 100);
    }

    public HasProgressOne(): boolean {
        return this.progress === 1;
    }
}

/**
 * REST API Wrapper for /api/progress
 */
const Progress = {
    getBatch(ids: number[]) {
        const query = "[]ids=" + ids.join("&[]ids=");
        return get("/api/progress/streams?" + query).then((progresses: ProgressItem[]) => {
            return progresses.map((p) => new ProgressItem(p)); // Recreate for Percentage(),...
        });
    },
};

/**
 * Wrapper for Javascript's fetch function
 * @param  {string} url URL to fetch
 * @param  {object} default_resp Return value in case of error
 * @return {Promise<Response>}
 */
async function get(url: string, default_resp: object = []) {
    return fetch(url)
        .then((res) => {
            if (!res.ok) {
                throw Error(res.statusText);
            }
            return res.json();
        })
        .catch((err) => {
            console.error(err);
            return default_resp;
        })
        .then((o) => o);
}
