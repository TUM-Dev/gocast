import { postData } from "./global";

export function createToken(expires: string, scope: string) {
    const req = {
        expires: null,
        scope: scope,
    };
    if (expires !== "") {
        const dateObj = new Date(expires);
        req.expires = dateObj.toISOString();
    }
    return postData("/api/token/create", req);
}

export function deleteToken(id: number) {
    return fetch(`/api/token/${id}`, {
        method: "DELETE",
    });
}
