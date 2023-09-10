export interface DirtyState {
    isDirty: boolean;
    dirtyKeys: string[];
}

/**
 * ## ChangeSet Class
 *
 * The `ChangeSet` class is designed to manage and track changes to a state object.
 * It provides an encapsulated way of observing state changes, committing them,
 * or rolling them back. Essentially, it helps in maintaining two versions of a state:
 * one that represents the current, committed state and another that captures all the changes (dirty state).
 *
 * ### Features
 * - **DirtyState**: Utilizes a `DirtyState` object to indicate if the state is dirty (modified but not committed)
 *   and which keys in the state object are dirty.
 * - **Custom Comparators**: Optionally, you can pass in a custom comparator function to determine how to compare state objects.
 * - **Event Subscriptions**: Offers an API for listening to changes in the state.
 *
 * ### Example Usage
 * ```typescript
 * const myState = { key1: 'value1', key2: 'value2' };
 * const changeSet = new ChangeSet(myState);
 *
 * changeSet.listen((changeState, dirtyState) => {
 *   console.log("Changed State:", changeState);
 *   console.log("Is Dirty:", dirtyState.isDirty);
 * });
 *
 * changeSet.patch('key1', 'new_value1');
 * ```
 *
 * ### Methods
 * - `listen(onUpdate)`: Subscribe to state changes.
 * - `removeListener(onUpdate)`: Unsubscribe from state changes.
 * - `get()`: Get the current uncommitted state.
 * - `set(val)`: Set the change state.
 * - `patch(key, val, options)`: Patch a specific key in the state object.
 * - `updateState(state)`: Update the state object, and reconcile it with the change state.
 * - `commit(options)`: Commit the change state, making it the new committed state.
 * - `reset()`: Reset the change state back to the last committed state.
 * - `isDirty()`: Check if the state has changes that are not yet committed.
 * - `changedKeys()`: Get the keys that have uncommitted changes.
 *
 * ### Alpine.js `bind-change-set` Directive Example
 * The `ChangeSet` class seamlessly integrates with Alpine.js through the `bind-change-set` directive.
 * For example, to bind a text input element to a `ChangeSet` instance, you can use the following HTML snippet:
 *
 * ```html
 * <input type="text" name="firstName" x-bind-change-set="changeSet">
 * ```
 *
 * This makes it a useful utility for handling state changes in a predictable way.
 */
export class ChangeSet<T> {
    private state: T;
    private changeState: T;
    private readonly comparator?: LectureComparator<T>;
    private onUpdate: ((changeState: T, dirtyState: DirtyState) => void)[];

    constructor(
        state: T,
        comparator?: (key: string, a: T, b: T) => boolean,
        onUpdate?: (changeState: T, dirtyState: DirtyState) => void,
    ) {
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
        this.changeState = { ...val };
        this.dispatchUpdate();
    }

    /**
     * Patches a key with a new value. This makes the state dirty.
     * @param key
     * @param val
     * @param isCommitted if true, the data will be passed also to the state, and won't make the model dirty.
     */
    /* eslint-disable @typescript-eslint/no-explicit-any */
    patch(key: string, val: any, { isCommitted = false }: { isCommitted?: boolean } = {}) {
        this.changeState = { ...this.changeState, [key]: val };
        if (isCommitted) {
            this.state = { ...this.state, [key]: val };
        }
        this.dispatchUpdate();
    }

    /**
     * Updates the state. Also updates all keys that are not dirty on the change-state, so they remain "undirty".
     * @param state
     */
    updateState(state: T) {
        const changedKeys = this.changedKeys();
        this.state = { ...state };

        for (const key of Object.keys(this.state)) {
            if (!changedKeys.includes(key)) {
                this.changeState[key] = this.state[key];
            }
        }

        this.dispatchUpdate();
    }

    /**
     * Commits the change state to the state. State is updated to the latest change state afterwards.
     * @param discardKeys List of keys that should be discarded and not committed.
     */
    commit({ discardKeys = [] }: { discardKeys?: string[] } = {}): void {
        for (const key in discardKeys) {
            this.changeState[key] = this.state[key];
        }
        this.state = { ...this.changeState };
        this.dispatchUpdate();
    }

    /**
     * Resets the change state to the state. Change state is the most current state afterwards.
     */
    reset(): void {
        this.changeState = { ...this.state };
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

export type LectureComparator<T> = (key: string, a: T, b: T) => boolean | null;

export function ignoreKeys<T>(list: string[]): LectureComparator<T> {
    return (key: string, a, b) => {
        if (list.includes(key)) {
            return false;
        }
        return null;
    }
}