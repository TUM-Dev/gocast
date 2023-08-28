import { StreamableMapProvider } from "./provider";
import {AdminLectureList, Lecture, UpdateLectureMetaRequest} from "../api/admin-lecture-list";

export class AdminLectureListProvider extends StreamableMapProvider<number, Lecture[]> {
    protected async fetcher(courseId: number): Promise<Lecture[]> {
        const result = await AdminLectureList.get(courseId);
        return result;
    }

    async add(courseId: number, lecture: Lecture): Promise<void> {
        await AdminLectureList.add(courseId, lecture);
        await this.fetch(courseId, true);
        await this.triggerUpdate(courseId);
    }

    async delete(courseId: number, lectureIds: number[]) {
        await AdminLectureList.delete(courseId, lectureIds);
        this.data[courseId] = (await this.getData(courseId)).filter((s) => !lectureIds.includes(s.ID));
        await this.triggerUpdate(courseId);
    }

    async updateMeta(courseId: number, streamId: number, request: UpdateLectureMetaRequest) {
        await AdminLectureList.update(courseId, streamId, request);

        this.data[courseId] = (await this.getData(courseId)).map((s) => {
            if (s.ID === streamId) {
                s = {
                    ...s,
                };

                // Path updated keys in local data
                for (const requestKey in request) {
                    const val = request[requestKey];
                    if (val !== undefined) {
                        s[requestKey] = val;
                    }
                }
            }
            return s;
        });
        await this.triggerUpdate(streamId);
    }
}
