export class audit {
    id: number;
    createdAt: string;
    message: string;
    type: string;
    userID: number;
    userName: string;
}

export function audits(offset: number, limit: number): Promise<audit[]> {
    return fetch(`/api/audits?offset=${offset}&limit=${limit}`).then((r) => {
        return r.json() as Promise<audit[]>;
    });
}
