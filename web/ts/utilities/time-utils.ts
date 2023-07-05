export const same_day = (a: Date, b: Date) =>
    a.getDate() === b.getDate() && a.getMonth() == b.getMonth() && a.getFullYear() === b.getFullYear();
