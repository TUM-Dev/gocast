export interface DirtyState {
    isDirty: boolean;
    dirtyKeys: string[];
}

export class ChangeSet<T> {
    private state: T;
    private changeState: T;
    private readonly comparator?: (key: string, a: T, b: T) => boolean|null;
    private onUpdate: ((changeState: T, dirtyState: DirtyState) => void)[];

    constructor(state: T, comparator?: (key: string, a: T, b: T) => boolean, onUpdate?: (changeState: T, dirtyState: DirtyState) => void) {
        this.state = state;
        this.onUpdate = onUpdate ? [onUpdate] : [];
        this.comparator = comparator;
        this.reset();
    }

    listen(onUpdate: (changeState: T, dirtyState: DirtyState) => void) {
        this.onUpdate.push(onUpdate);
    }

    removeListener(onUpdate: (changeState: T, dirtyState: DirtyState) => void) {
        this.onUpdate = this.onUpdate.filter((o) => o !== onUpdate);
    }

    get(): T {
        return this.changeState;
    }

    set(val: T) {
        this.changeState = {...val};
        this.dispatchUpdate();
    }

    /**
     * Patches a key with a new value. This makes the state dirty.
     * @param key
     * @param val
     * @param isCommitted if true, the data will be passed also to the state, and won't make the model dirty.
     */
    patch(key: string, val: any, { isCommitted = false }: { isCommitted : boolean }) {
        this.changeState = {...this.changeState, [key]: val};
        if (isCommitted) {
            this.state = {...this.state, [key]: val};
        }
        this.dispatchUpdate();
    }

    commit(): void {
        this.state = {...this.changeState};
    }

    reset(): void {
        this.changeState = {...this.state};
        this.dispatchUpdate();
    }

    isDirty(): boolean {
        for (const key of Object.keys(this.state)) {
            if (this.keyChanged(key)) {
                return true;
            }
        }
        return false;
    }

    changedKeys(): string[] {
        const res = [];
        for (const key of Object.keys(this.state)) {
            if (this.keyChanged(key)) {
                res.push(key);
            }
        }
        return res;
    }

    keyChanged(key: string): boolean {
        // Check with custom comparator if set
        if (this.comparator !== undefined) {
            const result = this.comparator(key, this.state, this.changeState);
            if (result !== null) {
                return result;
            }
        }

        // Otherwise just check if equiv
        if (this.state[key] !== this.changeState[key]) {
            return true;
        }
        return false;
    }

    dispatchUpdate() {
        if (this.onUpdate.length > 0) {
            const dirtyKeys = this.changedKeys();
            for (const onUpdate of this.onUpdate) {
                onUpdate(this.changeState, {
                    dirtyKeys,
                    isDirty: dirtyKeys.length > 0,
                });
            }
        }
    }
}
