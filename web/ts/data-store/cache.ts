export type CacheOptions = {
    validTime: number
};

type CacheEntry<T> = {
    createdAt: Date
    value: Promise<T>
};

export class Cache<T> {
    private options: CacheOptions;
    private cache: Map<string, CacheEntry<T>>;

    constructor(options: CacheOptions) {
        this.options = options;
        this.cache = new Map<string, CacheEntry<T>>();
    }

    public get(key: string, fn: () => Promise<T>, forceCacheRefresh: boolean = false) : Promise<T> {
        const entry = this.cache.get(key);
        if (!forceCacheRefresh && this.entryValid(entry)) {
            return entry.value;
        }

        const newEntry : CacheEntry<T> = {
            createdAt: new Date(),
            value: fn(),
        }
        this.cache.set(key, newEntry);

        return newEntry.value;
    }

    private entryValid(entry: CacheEntry<T>) : boolean {
        if (!entry) return false;
        const entryAge = Date.now() - entry.createdAt.getTime();
        return entryAge <= this.options.validTime;
    }
}