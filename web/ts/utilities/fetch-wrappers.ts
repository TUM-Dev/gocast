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

export interface PostFormDataListener {
    onProgress?: (progress: number) => void;
}

/**
 * Wrapper for XMLHttpRequest to send form data via POST
 * @param  {string} url URL to send to
 * @param  {FormData} body form-data
 * @param  {PostFormDataListener} listener attached progress listeners
 * @return {Promise<XMLHttpRequest>}
 */
export function postFormData(
    url: string,
    body: FormData,
    listener: PostFormDataListener = {},
): Promise<XMLHttpRequest> {
    const xhr = new XMLHttpRequest();
    return new Promise((resolve, reject) => {
        xhr.onloadend = () => {
            if (xhr.status === 200) {
                resolve(xhr);
            } else {
                reject(xhr);
            }
        };
        xhr.upload.onprogress = (e: ProgressEvent) => {
            if (!e.lengthComputable) {
                return;
            }
            if (listener.onProgress) {
                listener.onProgress(Math.floor(100 * (e.loaded / e.total)));
            }
        };
        xhr.open("POST", url);
        xhr.send(body);
    });
}

/**
 * Wrapper for Javascript's fetch function for PUT
 * @param  {string} url URL to fetch
 * @param  {object} body Data object to put
 * @return {Promise<Response>}
 */
export async function put(url = "", body: object = {}) {
    return await fetch(url, {
        method: "PUT",
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

/**
 * Wrapper for Javascript's fetch function for PATCH
 * @param  {string} url URL to fetch
 * @param  {object} body Data object to put
 * @return {Promise<Response>}
 */
export async function patch(url = "", body = {}) {
    return await fetch(url, {
        method: "PATCH",
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

/**
 * Wrapper for Javascript's fetch function for DELETE
 * @param  {string} url URL to fetch
 * @return {Promise<Response>}
 */
export async function del(url: string) {
    return await fetch(url, { method: "DELETE" });
}

/**
 * Wrapper for XMLHttpRequest to upload a file
 * @param url URL to upload to
 * @param file File to be uploaded
 * @param listener Upload progress listeners
 */
export function uploadFile(url: string, file: File, listener: PostFormDataListener = {}): Promise<XMLHttpRequest> {
    const vodUploadFormData = new FormData();
    vodUploadFormData.append("file", file);
    return postFormData(url, vodUploadFormData, listener);
}
