import { Realtime, RealtimeMessageTypes } from "../socket";

const channel = "live-runner-page-update";

export const liveRunnerUpdateListener = {
    async init() {
        await Realtime.get().subscribeChannel(channel, this.handle);
    },

    async handle(payload: object) {
        switch (payload["task"]) {
            case "aliveStatusUpdate": {
                const runners = {};
                for (const runner of payload["statuses"]) {
                    runners[runner["runner"]] = { alive: runner["status"], jobCount: runner["jobCount"] };
                    // console.log(runner)
                }
                // console.log("Dispatching event", runners);
                window.dispatchEvent(
                    new CustomEvent("runner-alive-status-update", {
                        detail: {
                            runners: runners,
                        },
                    }),
                );
                break;
            }
            default:
                console.log("Unknown task", payload);
                break;
        }
        //console.log(payload);
        // await Realtime.get().send(channel, {
        //     type: RealtimeMessageTypes.RealtimeMessageTypeChannelMessage,
        //     payload: { test: "test" },
        // });
    },

    async askForAliveStatus() {
        await Realtime.get().send(channel, {
            type: RealtimeMessageTypes.RealtimeMessageTypeChannelMessage,
            payload: { task: "aliveStatusUpdate" },
        });
    },
};

export async function periodicAskForAliveStatus() {
    setInterval(() => {
        liveRunnerUpdateListener.askForAliveStatus();
    }, 5000);
}

// ------------------------------------------------- REST API CALLS ------------------------------------
/** Make DELETE call to /api/workers/:id with given worker-id */
export async function deleteRunner(hostname: string) {
    return await fetch("/api/runners/" + hostname, {
        method: "DELETE",
    });
}
