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

    /**
     * Add listener to receive change set updates
     * @param onUpdate
     */
    listen(onUpdate: (changeState: T, dirtyState: DirtyState) => void) {
        this.onUpdate.push(onUpdate);
    }

    /**
     * Remove listener from change set.
     * @param onUpdate
     */
    removeListener(onUpdate: (changeState: T, dirtyState: DirtyState) => void) {
        this.onUpdate = this.onUpdate.filter((o) => o !== onUpdate);
    }

    /**
     * Returns the current uncommitted change state.
     */
    get(): T {
        return this.changeState;
    }

    /**
     * Sets the change state.
     * @param val
     */
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

    /**
     * Commits the change state to the state. State is updated to the latest change state afterwards.
     */
    commit(): void {
        this.state = {...this.changeState};
    }

    /**
     * Resets the change state to the state. Change state is the most current state afterwards.
     */
    reset(): void {
        this.changeState = {...this.state};
        this.dispatchUpdate();
    }

    /**
     * A flag that indicated whether the change state is the same then the state or not.
     */
    isDirty(): boolean {
        for (const key of Object.keys(this.state)) {
            if (this.keyChanged(key)) {
                return true;
            }
        }
        return false;
    }

    /**
     * Returns the keys that are not the same between the state and the change state.
     */
    changedKeys(): string[] {
        const res = [];
        for (const key of Object.keys(this.state)) {
            if (this.keyChanged(key)) {
                res.push(key);
            }
        }
        return res;
    }

    /**
     * Checks if a specific key's value is different on the state and the change state.
     * @param key Key that should be checked
     */
    keyChanged(key: string): boolean {
        // Check with custom comparator if set
        if (this.comparator !== undefined) {
            const result = this.comparator(key, this.state, this.changeState);
            if (result !== null) {
                return result;
            }
        }

        // else just check if equiv
        return this.state[key] !== this.changeState[key];
    }

    /**
     * Executes all onUpdate listeners
     */
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
