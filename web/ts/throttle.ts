// eslint-disable-next-line @typescript-eslint/no-explicit-any
export type ThrottleFunc = (...args: any[]) => void;

export function throttle<T>(fun: ThrottleFunc, delay = 100, skipFirst: boolean = false): ThrottleFunc {
    let lastInstance: NodeJS.Timeout | null = null;
    let first: boolean = true;

    return (...args): void => {
        if (skipFirst && first) {
            first = false;
            fun(...args);
            return;
        }

        if (lastInstance !== null) {
            clearTimeout(lastInstance);
        }
        lastInstance = setTimeout(() => {
            fun(...args);
        }, delay);
    };
}
