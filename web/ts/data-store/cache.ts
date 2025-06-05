export type CacheOptions = {
    validTime: number;
};

type CacheEntry<T> = {
    createdAt: Date;
    value: Promise<T>;
};

/**
 Cache is a simple implementation of a cache that stores the results of functions
 that return a Promise, and provides those results if they are requested again within a
 certain timeframe.
 */
export class Cache<T> {
    private options: CacheOptions;
    private cache: Map<string, CacheEntry<T>>;

    /**
     Constructs a new Cache object with the specified options.
     @param options - The options for the Cache.
     */
    constructor(options: CacheOptions) {
        this.options = options;
        this.cache = new Map<string, CacheEntry<T>>();
    }

    /**
     Gets the cached value associated with the specified key, or invokes the provided function
     to retrieve the value and caches it for future requests.
     @param key - The key for the cached value.
     @param fn - The function to invoke if the value is not already cached.
     @param forceCacheRefresh - Whether to force fetch the value again.
     @returns A Promise that resolves with the cached value.
     */
    public get(key: string, fn: () => Promise<T>, forceCacheRefresh = false): Promise<T> {
        if (this.options.validTime === 0) {
            return fn();
        }

        const entry = this.cache.get(key);
        if (!forceCacheRefresh && this.entryValid(entry)) {
            return entry.value;
        }

        const newEntry: CacheEntry<T> = {
            createdAt: new Date(),
            value: fn(),
        };
        this.cache.set(key, newEntry);

        return newEntry.value;
    }

    /**
     Checks whether the provided CacheEntry is still valid based on its creation date
     and the validTime option provided in the Cache constructor.
     @param entry - The CacheEntry to check.
     @returns A boolean indicating whether the CacheEntry is still valid.
     */
    private entryValid(entry: CacheEntry<T>): boolean {
        if (!entry) return false;
        const entryAge = Date.now() - entry.createdAt.getTime();
        return entryAge <= this.options.validTime;
    }
}
