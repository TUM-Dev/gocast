import { StatusCodes } from "http-status-codes";
import { retractMessage } from "./watch";

interface maintenancePage {
    generateThumbnails(): Promise<boolean>;

    running: boolean;
    process: number;

    keepUpdated(): void;
    update(): void;

    cronJobs: string[];
    selectedCronJob: string;
    fetchCronJobs(): void;
    runSelectedCronJob(): Promise<boolean>;
    cronRunOk: boolean | null;
}

export function maintenancePage(): maintenancePage {
    return {
        generateThumbnails() {
            return fetch("/api/maintenance/generateThumbnails", { method: "POST" }).then((r) => {
                return true;
            });
        },
        running: false,
        process: 0,
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
                    return r.json() as Promise<{ process: number; running: boolean }>;
                })
                .then((r) => {
                    this.running = r.running;
                    this.process = r.process;
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
                    setTimeout(()=>{ this.cronRunOk=null; }, 5000)
                    this.cronRunOk = ok;
                    if (ok) {
                        this.selectedCronJob = "---";
                    }
                    return ok;
                });
        },
        cronRunOk: null,
    };
}
