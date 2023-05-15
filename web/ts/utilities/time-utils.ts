export const date_eq = (a: Date, b: Date) =>
    a.getDate() === b.getDate() && a.getMonth() == b.getMonth() && a.getFullYear() === b.getFullYear();
