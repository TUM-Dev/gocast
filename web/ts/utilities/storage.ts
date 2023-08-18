export function setInStorage(key: string, value: string, storage = window.localStorage) {
    storage.setItem(key, value);
}

export function getFromStorage(key: string, storage = window.localStorage): string {
    return storage.getItem(key);
}
