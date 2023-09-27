export type ThrottleFunc = ((...args: any[]) => void)

export function throttle<T>(fun: ThrottleFunc, delay = 100): ThrottleFunc {
    let lastInstance: NodeJS.Timeout|null = null;

    return (...args): void => {
        if (lastInstance !== null) {
            clearTimeout(lastInstance);
        }
        lastInstance = setTimeout(() => {
            fun(...args);
        }, delay);
    };
}