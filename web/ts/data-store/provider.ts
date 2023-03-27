import { ValueListener, ValueStreamMap } from "../value-stream";

export abstract class StreamableMapProvider<K, T> {
    protected data: Map<string, Promise<T>> = new Map<string, Promise<T>>();
    protected stream: ValueStreamMap<T> = new ValueStreamMap<T>();

    protected async triggerUpdate(key: K) {
        this.stream.add(key.toString(), await this.data[key.toString()]);
    }

    async subscribe(key: K, callback: ValueListener<T>): Promise<void> {
        if (this.data[key.toString()] == null) {
            await this.fetch(key);
        }

        this.stream.subscribe(key.toString(), callback);
        await this.triggerUpdate(key);
    }

    unsubscribe(key: K, callback: ValueListener<T>): void {
        this.stream.unsubscribe(key.toString(), callback);
    }

    protected abstract fetch(key: K);
}
