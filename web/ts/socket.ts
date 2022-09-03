const WS_INITIAL_RETRY_DELAY = 5000;
const PAGE_LOADED = new Date();

type MessageHandlerFn = (payload: object) => void;

const RealtimeMessageTypes = {
    RealtimeMessageTypeSubscribe: "subscribe",
    RealtimeMessageTypeUnsubscribe: "unsubscribe",
    RealtimeMessageTypeChannelMessage: "message",
};

export class Realtime {
    private debugging = false;
    private ws: WebSocket;
    private handler: object = {};

    // Singleton
    private static instance;
    static get(): Realtime {
        if (this.instance == null) this.instance = new Realtime();
        return this.instance;
    }

    public init() {
        return this.connect(WS_INITIAL_RETRY_DELAY);
    }

    async send(channel: string, { payload = {}, type = RealtimeMessageTypes.RealtimeMessageTypeChannelMessage }) {
        await this.lazyInit();
        await this.ws.send(
            JSON.stringify({
                type: type,
                channel: channel,
                payload: payload,
            }),
        );
        this.debug("🔵 Send", { type, channel, payload });
    }

    public async subscribeChannel(channel: string, handler?: MessageHandlerFn) {
        await this.lazyInit();
        if (handler) this.registerHandler(channel, handler);
        await this.send(channel, {
            type: RealtimeMessageTypes.RealtimeMessageTypeSubscribe,
        });
        this.debug("Subscribed", channel);
    }

    public async unsubscribeChannel(channel: string, { unregisterHandler = true }) {
        await this.lazyInit();
        if (unregisterHandler) {
            delete this.handler[channel];
        }
        await this.send(channel, {
            type: RealtimeMessageTypes.RealtimeMessageTypeUnsubscribe,
        });
        this.debug("Unsubscribed", channel);
    }

    public registerHandler(channel: string, handler: MessageHandlerFn) {
        if (!this.handler[channel]) this.handler[channel] = [];
        this.handler[channel].push(handler);
    }

    public unregisterHandler(channel: string, handler: MessageHandlerFn) {
        if (this.handler[channel]) {
            this.handler[channel] = this.handler[channel].filter((fn) => fn === handler);
        }
    }

    private lazyInit() {
        if (this.ws) return;
        this.debug("lazy init");
        return this.init();
    }

    private handleMessage({ channel, payload }) {
        this.debug("⚪️️ Received", { channel, payload });
        if (this.handler[channel]) {
            for (const handler of this.handler[channel]) {
                handler(payload);
            }
        }
    }

    private debug(description: string, ...data) {
        if (!this.debugging) return;
        console.info("[WS_REALTIME_DEBUG]", description, ...data);
    }

    private triggerConnectionStatusEvent(status: boolean) {
        const event = new CustomEvent("wsrealtimeconnectionchange", { detail: { status } });
        window.dispatchEvent(event);
    }

    private async afterConnect(): Promise<void> {
        this.debug("connected");

        // Re-Subscribe to all channels
        for (const channel of Object.keys(this.handler)) {
            await this.send(channel, {
                type: RealtimeMessageTypes.RealtimeMessageTypeSubscribe,
            });
            this.debug("Re-Subscribed", channel);
        }
    }

    private connect(retryDelay: number): Promise<void> {
        return new Promise<void>((res, rej) => {
            let promiseDone = false;
            const wsProto = window.location.protocol === "https:" ? `wss://` : `ws://`;
            this.ws = new WebSocket(`${wsProto}${window.location.host}/api/pub-sub/ws`);
            this.ws.onopen = () => {
                this.afterConnect();
                this.triggerConnectionStatusEvent(true);
                if (!promiseDone) {
                    promiseDone = true;
                    res();
                }
            };

            this.ws.onmessage = (m) => {
                const data = JSON.parse(m.data);
                this.handleMessage(data);
            };

            this.ws.onclose = () => {
                this.triggerConnectionStatusEvent(false);
                this.debug("disconnected");
                // connection closed, discard old websocket and create a new one after backoff
                // don't recreate new connection if page has been loaded more than 12 hours ago
                if (new Date().valueOf() - PAGE_LOADED.valueOf() > 1000 * 60 * 60 * 12) {
                    return;
                }

                this.ws = null;
                setTimeout(
                    () => this.connect(retryDelay * 2), // Exponential Backoff
                    retryDelay,
                );
            };

            this.ws.onerror = (err) => {
                this.debug("error", err);
            };
        });
    }
}
