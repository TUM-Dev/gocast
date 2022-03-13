/** Make DELETE call to /api/workers/:id with given worker-id */
export async function deleteWorker(id: string) {
    return await fetch("/api/workers/" + id, {
        method: "DELETE",
    });
}
