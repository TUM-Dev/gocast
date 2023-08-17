export class SmartArray<T> {
    protected list: T[];

    constructor(list: T[]) {
        this.list = list;
    }

    get(sortFn?: CompareFunction<T>, filterPred?: FilterPredicate<T>): T[] {
        const copy = filterPred ? [...this.list].filter(filterPred) : [...this.list];
        return sortFn ? copy.sort(sortFn) : copy;
    }

    set(list: T[]): SmartArray<T> {
        this.list = list;
        return this;
    }

    forEach(callback: (obj: T, i: number) => void): SmartArray<T> {
        this.list.forEach(callback);
        return this;
    }

    hasElements() {
        return this.list.length > 0;
    }
}

export class GroupedSmartArray<T, K extends keyof any> {
    protected groups: Map<K, SmartArray<T>>;

    constructor(list: T[], key: (i: T) => K) {
        this.group(list, key);
    }

    get() {
        return this.groups;
    }

    set(list: T[], key: (i: T) => K): GroupedSmartArray<T, K> {
        this.group(list, key);
        return this;
    }

    hasElements() {
        return this.groups.size > 0;
    }

    private group(list: T[], key: (i: T) => K) {
        const map = new Map<K, SmartArray<T>>();
        const d: Record<K, T[]> = list.reduce((groups, item) => {
            (groups[key(item)] ||= []).push(item);
            return groups;
        }, {} as Record<K, T[]>);
        for (const [key, value] of Object.entries(d)) {
            // @ts-ignore
            map.set(key, new SmartArray<T>(value));
        }
        this.groups = map;
    }
}

export type CompareFunction<T> = (a: T, b: T) => number;
export type FilterPredicate<T> = (o: T) => boolean;
