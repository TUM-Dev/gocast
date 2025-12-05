import { StatusCodes } from "http-status-codes";

interface maintenancePage {
    generateThumbnails(): Promise<boolean>;

    running: boolean;
    progress: number;

    keepUpdated(): void;
    update(): void;

    cronJobs: string[];
    selectedCronJob: string;
    fetchCronJobs(): void;
    runSelectedCronJob(): Promise<boolean>;
    cronRunOk: boolean | null;

    fetchTranscodingFailures(): void;
    transcodingFailures: { ID: number }[];
    deleteTranscodingFailure(id: number): void;

    fetchEmailFailures(): void;
    emailFailures: { ID: number }[];
    deleteEmailFailure(id: number): void;
}

interface Theme {
    id: string;
    name: string;
    description: string;
    icon: string;
}

interface themePage {
    themes: Theme[];
    activeTheme: string;
    loading: boolean;
    error: string | null;
    success: boolean;
    fetchThemes(): void;
    setActiveTheme(themeId: string): void;
}

export function themePage(): themePage {
    return {
        themes: [],
        activeTheme: "",
        loading: true,
        error: null,
        success: false,
        fetchThemes() {
            this.loading = true;
            this.error = null;

            // Fetch available themes and active theme in parallel
            Promise.all([
                fetch("/api/theme/available").then((r) => r.json()),
                fetch("/api/theme/active").then((r) => r.json()),
            ])
                .then(([themes, activeResponse]) => {
                    this.themes = themes;
                    this.activeTheme = activeResponse.themeId || "";
                    this.loading = false;
                })
                .catch((err) => {
                    this.error = "Failed to load themes: " + err.message;
                    this.loading = false;
                });
        },
        setActiveTheme(themeId: string) {
            this.error = null;
            this.success = false;

            fetch("/api/theme/active", {
                method: "PUT",
                headers: {
                    "Content-Type": "application/json",
                },
                body: JSON.stringify({ themeId }),
            })
                .then((r) => {
                    if (r.status === StatusCodes.OK) {
                        this.activeTheme = themeId;
                        this.success = true;
                        // Auto-hide success message after 3 seconds
                        setTimeout(() => {
                            this.success = false;
                        }, 3000);
                    } else {
                        throw new Error("Failed to update theme");
                    }
                })
                .catch((err) => {
                    this.error = err.message;
                });
        },
    };
}

export function maintenancePage(): maintenancePage {
    return {
        generateThumbnails() {
            return fetch("/api/maintenance/generateThumbnails", { method: "POST" }).then((r) => {
                return true;
            });
        },
        running: false,
        progress: 0,
        keepUpdated() {
            this.update();
            setTimeout(() => {
                this.update();
                this.keepUpdated();
            }, 5000);
        },
        update() {
            fetch(`/api/maintenance/generateThumbnails/status`)
                .then((r) => {
                    return r.json() as Promise<{ progress: number; running: boolean }>;
                })
                .then((r) => {
                    this.running = r.running;
                    this.progress = r.progress;
                });
        },
        cronJobs: [],
        selectedCronJob: "",
        fetchCronJobs() {
            fetch("/api/maintenance/cron/available")
                .then((r) => r.json())
                .then((r) => (this.cronJobs = r));
        },
        runSelectedCronJob(): Promise<boolean> {
            return fetch("/api/maintenance/cron/run?job=" + this.selectedCronJob, { method: "POST" })
                .then((r) => r.status === StatusCodes.OK)
                .catch((r) => false)
                .then((ok) => {
                    // remove status text after 5 seconds
                    setTimeout(() => {
                        this.cronRunOk = null;
                    }, 5000);
                    this.cronRunOk = ok;
                    if (ok) {
                        this.selectedCronJob = "---";
                    }
                    return ok;
                });
        },
        cronRunOk: null,
        fetchTranscodingFailures() {
            fetch("/api/maintenance/transcodingFailures")
                .then((r) => r.json())
                .then((r) => (this.transcodingFailures = r));
        },
        transcodingFailures: [],
        deleteTranscodingFailure(id: number) {
            fetch("/api/maintenance/transcodingFailures/" + id, { method: "DELETE" }).then((r) => {
                if (r.status === StatusCodes.OK) {
                    console.log(id);
                    this.transcodingFailures = this.transcodingFailures.filter((f) => f.ID !== id);
                }
            });
        },
        fetchEmailFailures() {
            fetch("/api/maintenance/emailFailures")
                .then((r) => r.json())
                .then((r) => (this.emailFailures = r));
        },
        emailFailures: [],
        deleteEmailFailure(id: number) {
            fetch("/api/maintenance/emailFailures/" + id, { method: "DELETE" }).then((r) => {
                if (r.status === StatusCodes.OK) {
                    console.log(id);
                    this.emailFailures = this.emailFailures.filter((f) => f.ID !== id);
                }
            });
        },
    };
}
