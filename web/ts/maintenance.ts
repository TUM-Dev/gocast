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
    };
}
