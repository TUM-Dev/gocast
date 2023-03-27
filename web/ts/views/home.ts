export function header() {
    return {
        userContext: new Toggleable(),

        notifications: new Notifications(),
        notification: new Toggleable(),
        toggleNotification(set?: boolean) {
            this.notification.toggle(set);
            this.notifications.writeToStorage(true);
        },

        themePicker: new Toggleable(),
    };
}

export enum Views {
    Main,
    UserCourses,
    PublicCourses,
}

const DEFAULT_LECTURE_NAME = "Untitled lecture";

export function body() {
    const url = new URL(window.location.href);
    return {
        term: url.searchParams.get("term") ?? undefined,
        year: +url.searchParams.get("year"),
        view: +url.searchParams.get("view") ?? Views.Main,

        navigation: new Toggleable(),
        allSemesters: new Toggleable(),

        semesters: [],
        currentSemesterIndex: -1,
        selectedSemesterIndex: -1,

        publicCourses: [],
        userCourses: [],
        liveToday: [],
        recentVods: [],

        loadingIndicator: 0,

        init() {
            this.load([this.loadSemesters(), this.loadPublicCourses(), this.loadUserCourses()]);
        },

        load(promises: Promise<object>[]) {
            this.loadingIndicator = 0;
            Promise.all(promises).then(() => {
                this.loadingIndicator = 0;
            });
            promises.forEach((p) => {
                Promise.resolve(p).then((r) => (this.loadingIndicator += 100 / promises.length));
            });
        },

        onPopState(event: PopStateEvent) {
            const proxy = new Proxy(event.state || {}, {
                get(target, p, receiver) {
                    return target[p] || undefined;
                },
            });
            this.switchSemester(+proxy.year || 0, proxy.term);
        },

        showMain() {
            this.view = Views.Main;
            this.navigation.toggle(false);
        },

        showUserCourses() {
            this.view = Views.UserCourses;
            this.navigation.toggle(false);
        },

        showPublicCourses() {
            this.view = Views.PublicCourses;
            this.navigation.toggle(false);
        },

        async loadSemesters() {
            const res: SemesterResponse = await Semesters.get();
            this.semesters = res.Semesters;
            this.currentSemester = res.Current;
            this.currentSemesterIndex = this.semesters.findIndex(
                (s) => this.currentSemester.Year === s.Year && this.currentSemester.TeachingTerm === s.TeachingTerm,
            );

            if (this.year !== null && this.term != null) {
                this.selectedSemesterIndex = this.semesters.findIndex(
                    (s) => this.year === s.Year && this.term === s.TeachingTerm,
                );
            }

            if (this.selectedSemesterIndex === -1) {
                this.selectedSemesterIndex = this.currentSemesterIndex;
            }
        },

        async loadPublicCourses() {
            this.publicCourses = await Courses.getPublic(this.year, this.term);
        },

        async loadUserCourses() {
            this.userCourses = await Courses.getUsers(this.year, this.term);
            this.recentVods = this.getRecentVods();
            this.liveToday = this.getLiveToday();
        },

        getLiveToday() {
            return this.userCourses.filter((c) => {
                if (c.nextLecture.ID !== 0) {
                    const start = new Date(c.nextLecture.Start);
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

        getRecentVods() {
            const courses = this.userCourses.filter((c) => c.lastLecture.ID !== 0);
            courses.forEach(async (c) => {
                c.lastLecture.Name = c.lastLecture.Name === "" ? DEFAULT_LECTURE_NAME : c.lastLecture.Name;
                c.lastLecture.Progress = await Progress.get(c.lastLecture.ID);
            });
            return courses;
        },

        async switchSemester(year?: number, term?: string) {
            console.log({ year, term });
            if (year !== undefined && term !== undefined) {
                if (this.year !== year || this.term !== term) {
                    this.year = year;
                    this.term = term;
                    this.selectedSemesterIndex = this.semesters.findIndex(
                        (s) => s.Year === year && s.TeachingTerm === term,
                    );
                    this.allSemesters.toggle(false);

                    url.searchParams.set("year", String(year));
                    url.searchParams.set("term", term);
                    window.history.pushState({ year, term }, "", url.toString());
                }
            }
            this.load([this.loadPublicCourses(), this.loadUserCourses()]);
        },
    };
}

export function main() {
    return {
        livestreams: [],
        async loadLivestreams() {
            this.livestreams = await Courses.getLivestreams();
        },
    };
}

class Toggleable {
    public value: boolean;
    constructor(value = false) {
        this.value = value;
    }
    toggle(set?: boolean) {
        this.value = set || !this.value;
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

type SemesterItem = {
    TeachingTerm: string;
    Year: number;
    FriendlyString?: string;
};

const Semesters = {
    async get(): Promise<SemesterResponse> {
        return get("/api/semesters").then((l: SemesterResponse) => {
            l.Semesters.forEach(
                (s: SemesterItem) => (s.FriendlyString = `${s.TeachingTerm === "W" ? "Winter" : "Summer"} ${s.Year}`),
            );
            return l;
        });
    },
};

const Courses = {
    async getLivestreams() {
        return get("/api/courses/live").then((livestreams) => {
            // force them to use titles...
            livestreams.forEach((l) => {
                l.Stream.Name = l.Stream.Name === "" ? DEFAULT_LECTURE_NAME : l.Stream.Name;

                const end = new Date(l.Stream.End);
                const hours = end.getHours();
                const minutes = end.getMinutes();
                l.Stream.FriendlyDateString = `Until ${hours}:${minutes < 10 ? minutes + "0" : minutes}`;

                return l;
            });
            return livestreams;
        });
    },

    async getPublic(year?: number, term?: string): Promise<object> {
        const query = year !== undefined && term !== undefined && year !== 0 ? `?year=${year}&term=${term}` : "";
        return get(`/api/courses/public${query}`);
    },

    async getUsers(year?: number, term?: string): Promise<object> {
        const query = year !== undefined && term !== undefined && year !== 0 ? `?year=${year}&term=${term}` : "";
        return get(`/api/courses/users${query}`);
    },
};

type Progress = {
    progress: number;
    percentage?: number;
    watched: boolean;
    streamId: number;
};

const Progress = {
    get(streamId: number) {
        return get("/api/progress/streams/" + streamId).then((p: Progress) => {
            return { ...p, percentage: Math.round(p.progress * 100) };
        });
    },
};

function get(url: string, default_resp: object = []) {
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
