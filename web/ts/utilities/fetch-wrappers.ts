/**
 * Wrapper for Javascript's fetch function for GET
 * @param  {string} url URL to fetch
 * @param  {object} default_resp Return value in case of error
 * @return {Promise<Response>}
 */
export async function get(url: string, default_resp: object = [], throw_err = false) {
    return fetch(url)
        .then((res) => {
            if (!res.ok) {
                throw Error(res.statusText);
            }
            return res.json();
        })
        .catch((err) => {
            if (!throw_err) {
                return default_resp;
            }
            throw err;
        })
        .then((o) => o);
}

/**
 * Wrapper for Javascript's fetch function for POST
 * @param  {string} url URL to fetch
 * @param  {object} body Data object to post
 * @return {Promise<Response>}
 */
export async function post(url: string, body: object = {}) {
    return fetch(url, {
        method: "POST",
        headers: {
            "Content-Type": "application/json",
        },
        body: JSON.stringify(body),
    }).then((res) => {
        if (!res.ok) {
            throw Error(res.statusText);
        }
        return res;
    });
}
