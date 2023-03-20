export function header() {
    return {
        showUserContext: false,
        toggleUserContext(set?: boolean) {
            this.showUserContext = set || !this.showUserContext;
        },

        notifications: new Notifications(),
        showNotifications: false,
        toggleNotifications(set?: boolean) {
            this.showNotifications = set || !this.showNotifications;
            this.notifications.writeToStorage(true);
        },

        showThemePicker: false,
        toggleThemePicker(set?: boolean) {
            this.showThemePicker = set || !this.showThemePicker;
        },
    };
}

export enum Views {
    Main,
    UserCourses,
    PublicCourses,
}

const DEFAULT_LECTURE_NAME = "Untitled lecture";

export function body() {
    return {
        currentView: Views.Main,
        showMain() {
            this.currentView = Views.Main;
        },

        showUserCourses() {
            this.currentView = Views.UserCourses;
        },

        showPublicCourses() {
            this.currentView = Views.PublicCourses;
        },

        showNavigation: false,
        toggleNavigation(set?: boolean) {
            this.showNavigation = set || !this.showNavigation;
        },

        showAllSemesters: false,
        toggleAllSemesters(set?: boolean) {
            this.showAllSemesters = set || !this.showAllSemesters;
        },

        semesters: [],
        currentSemesterIndex: -1,
        selectedSemesterIndex: -1,
        async loadSemesters() {
            this.semesters = await Semesters.get();
        },
        async loadCurrentSemester() {
            this.currentSemester = await Semesters.getCurrent();
            this.currentSemesterIndex = this.semesters.findIndex(
                (s) => this.currentSemester.Year === s.Year && this.currentSemester.TeachingTerm === s.TeachingTerm,
            );
            this.selectedSemesterIndex = this.currentSemesterIndex;
        },

        publicCourses: [],
        async loadPublicCourses() {
            this.publicCourses = await Courses.getPublic();
        },

        userCourses: [],
        liveToday: [],
        recentVods: [],
        async loadUserCourses() {
            this.userCourses = await Courses.getUsers();
            this.recentVods = await this.getRecentVods();
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

        async switchSemester(year, term, semesterIndex) {
            this.publicCourses = await Courses.getPublic(year, term);
            this.userCourses = await Courses.getUsers(year, term);
            this.liveToday = this.getLiveToday();
            this.recentVods = this.getRecentVods();
            this.selectedSemesterIndex = semesterIndex;
            this.showAllSemesters = false;
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

const Semesters = {
    async get(): Promise<Semester[]> {
        return get("/api/semesters").then((l: Semester[]) => {
            l.forEach((s) => (s.FriendlyString = `${s.TeachingTerm === "W" ? "Winter" : "Summer"} ${s.Year}`));
            return l;
        });
    },

    async getCurrent(): Promise<Semester> {
        return get("/api/semesters/current");
    },
};

type Semester = {
    TeachingTerm: string;
    Year: number;
    FriendlyString?: string;
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
        const query = year !== undefined && term !== undefined ? `?year=${year}&term=${term}` : "";
        return get(`/api/courses/public${query}`);
    },

    async getUsers(year?: number, term?: string): Promise<object> {
        const query = year !== undefined && term !== undefined ? `?year=${year}&term=${term}` : "";
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
            p.percentage = Math.round(p.progress * 100);
            return p;
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
