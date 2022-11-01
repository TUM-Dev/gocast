import { Delete } from "./global";

enum FileType {
    file,
    url,
}

export const Files = {
    fileType: FileType,
    add: async function (request: FormData, lectureId: number, type: FileType): Promise<number> {
        const init = { method: "POST", body: request };
        return await fetch(`/api/stream/${lectureId}/files?type=${FileType[type]}`, init)
            .then((res) => {
                if (!res.ok) {
                    throw Error(res.statusText);
                }
                return res.json();
            })
            .catch((err) => {
                console.error(err);
            })
            .then((res) => res);
    },
    delete: async function (lectureId: number, fileId: number): Promise<Response> {
        return Delete(`/api/stream/${lectureId}/files/${fileId}`);
    },
};
