interface maintenancePage {
    generateThumbnails(): Promise<boolean>;

    running: boolean;
    process: number;

    keepUpdated(): void;
    update(): void;
}

export function maintenancePage(): maintenancePage {
    return {
        generateThumbnails() {
            return fetch("/api/maintenance/generateThumbnails", {method: "POST"}).then((r) => {
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
            fetch(
                `/api/maintenance/generateThumbnails/status`,
            )
                .then((r) => {
                    return r.json() as Promise<{ process: number, running: boolean }>;
                })
                .then((r) => {
                    this.running = r.running;
                    this.process = r.process;
                });
        }
    };
}

