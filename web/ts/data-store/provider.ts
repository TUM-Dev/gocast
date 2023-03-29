import { ValueListener, ValueStreamMap } from "../value-stream";
import { Cache } from "./cache";

/**
 An abstract class representing a provider for a streamable map.
 @template K - The type of the keys in the map.
 @template T - The type of the values in the map.
 */
export abstract class StreamableMapProvider<K, T> {
    protected data: Map<string, Promise<T>> = new Map<string, Promise<T>>();
    protected stream: ValueStreamMap<T> = new ValueStreamMap<T>();

    // 10 min cache time
    protected cache: Cache<T> = new Cache<T>({ validTime: 10 * 60 * 1000 });

    /**
     Fetches the data for the specified key and caches it if not already cached.
     @param key - The key for the data.
     @param force - Whether to force fetch the data.
     @returns A Promise that resolves when the data is fetched and cached.
     */
    protected async fetch(key: K, force = false): Promise<void> {
        this.data[key.toString()] = this.cache.get(`get.${key.toString()}`, () => this.fetcher(key), force);
        await this.data[key.toString()];
    }

    /**
     Retrieves the data for the specified key.
     If the data is not already cached, it will be fetched and cached.
     @param key - The key to retrieve the data for.
     @param forceFetch - Whether to force fetch the data.
     @returns A Promise that resolves with the data associated with the specified key.
     */
    async getData(key: K, forceFetch = false): Promise<T> {
        if (this.data[key.toString()] == null || forceFetch) {
            await this.fetch(key, forceFetch);
            await this.triggerUpdate(key);
        }
        return this.data[key.toString()];
    }

    /**
     Triggers an update to the specified key in the stream.
     @param key - The key to update in the stream.
     */
    protected async triggerUpdate(key: K) {
        this.stream.add(key.toString(), await this.data[key.toString()]);
    }

    /**
     Subscribes a callback to receive updates for the specified key in the stream.
     If the data is not already cached, it will be fetched and cached.
     @param key - The key to subscribe the callback to.
     @param callback - The callback function to subscribe.
     @returns A Promise that resolves when the subscription is complete.
     */
    async subscribe(key: K, callback: ValueListener<T>): Promise<void> {
        if (this.data[key.toString()] == null) {
            await this.fetch(key);
        }

        this.stream.subscribe(key.toString(), callback);
        await this.triggerUpdate(key);
    }

    /**
     Unsubscribes a callback from receiving updates for the specified key in the stream.
     @param key - The key to unsubscribe the callback from.
     @param callback - The callback function to unsubscribe.
     */
    unsubscribe(key: K, callback: ValueListener<T>): void {
        this.stream.unsubscribe(key.toString(), callback);
    }

    /**
     An abstract method to be implemented by subclasses to fetch the data for the specified key.
     @param key - The key for the data to be fetched.
     @returns A Promise that resolves with the fetched data.
     */
    protected abstract fetcher(key: K): Promise<T>;
}
