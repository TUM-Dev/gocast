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

export function sideNavigation() {
    return {
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
        getSlicedSemesters(): Semester[] {
            return this.showAllSemesters ? this.semesters : this.semesters.slice(0, 3);
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
    get: async function (): Promise<Semester[]> {
        return fetch("/api/semesters")
            .then((res) => {
                if (!res.ok) {
                    throw Error(res.statusText);
                }

                return res.json();
            })
            .catch((err) => {
                console.error(err);
                return [];
            })
            .then((l: Semester[]) => {
                l.forEach((s) => (s.FriendlyString = `${s.TeachingTerm === "W" ? "Winter" : "Summer"} ${s.Year}`));
                return l;
            });
    },

    getCurrent(): Promise<Semester> {
        return fetch("/api/semesters/current")
            .then((res) => {
                if (!res.ok) {
                    throw Error(res.statusText);
                }

                return res.json();
            })
            .catch((err) => {
                console.error(err);
                return [];
            })
            .then((s) => s);
    },
};

type Semester = {
    TeachingTerm: string;
    Year: number;
    FriendlyString?: string;
};
