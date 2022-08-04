const WS_INITIAL_RETRY_DELAY = 5000;
const PAGE_LOADED = new Date();

type MessageHandlerFn = (payload: object) => void;

const RealtimeMessageTypes = {
    RealtimeMessageTypeSubscribe: "subscribe",
    RealtimeMessageTypeUnsubscribe: "unsubscribe",
    RealtimeMessageTypeChannelMessage: "message",
};

export const realtime = {
    _debugging: true,
    _ws: null,
    _handler: {},

    init() {
        return this._connect(WS_INITIAL_RETRY_DELAY);
    },

    async send(channel: string, { payload = {}, type = RealtimeMessageTypes.RealtimeMessageTypeChannelMessage }) {
        await this._lazyInit();
        this._ws.send(
            JSON.stringify({
                type: type,
                channel: channel,
                payload: payload,
            }),
        );
    },

    async subscribeChannel(channel: string, handler?: MessageHandlerFn) {
        if (handler) this.registerHandler(channel, handler);
        await this.send(channel, {
            type: RealtimeMessageTypes.RealtimeMessageTypeSubscribe,
        });
    },

    async unsubscribeChannel(channel: string, { unregisterHandler = true }) {
        if (unregisterHandler) {
            delete this._handler[channel];
        }
        await this.send(channel, {
            type: RealtimeMessageTypes.RealtimeMessageTypeUnsubscribe,
        });
    },

    registerHandler(channel: string, handler: MessageHandlerFn) {
        if (!this._handler[channel]) this._handler[channel] = [];
        this._handler[channel].push(handler);
    },

    unregisterHandler(channel: string, handler: MessageHandlerFn) {
        if (this._handler[channel]) {
            this._handler[channel] = this._handler[channel].filter((fn) => fn === handler);
        }
    },

    _lazyInit() {
        if (this._ws) return;
        this._debug("lazy init");
        return this.init();
    },

    _handleMessage({ channel, payload }) {
        this._debug("received message ", { channel, payload });
        if (this._handler[channel]) {
            for (const handler of this._handler[channel]) {
                handler(payload);
            }
        }
    },

    _debug(description: string, ...data) {
        if (!this._debugging) return;
        console.info("[WS_REALTIME_DEBUG]", description, ...data);
    },

    _connect(retryDelay: number) {
        return new Promise<void>((res, rej) => {
            let promiseDone = false;
            const wsProto = window.location.protocol === "https:" ? `wss://` : `ws://`;
            this._ws = new WebSocket(`${wsProto}${window.location.host}/api/pub-sub/ws`);
            this._ws.onopen = (e) => {
                this._debug("connected", e);
                if (!promiseDone) {
                    promiseDone = true;
                    res();
                }
            };

            this._ws.onmessage = (m) => {
                const data = JSON.parse(m.data);
                this._handleMessage(data);
            };

            this._ws.onclose = () => {
                this._debug("disconnected");
                // connection closed, discard old websocket and create a new one after backoff
                // don't recreate new connection if page has been loaded more than 12 hours ago
                if (new Date().valueOf() - PAGE_LOADED.valueOf() > 1000 * 60 * 60 * 12) {
                    return;
                }

                this._ws = null;
                setTimeout(
                    () => this._connect(retryDelay * 2), // Exponential Backoff
                    retryDelay,
                );
            };

            this._ws.onerror = (err) => {
                this._debug("error", err);
            };
        });
    },
};
