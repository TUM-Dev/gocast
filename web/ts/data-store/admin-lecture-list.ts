import { StreamableMapProvider } from "./provider";
import {AdminLectureList, Lecture, UpdateLectureMetaRequest} from "../api/admin-lecture-list";
import {FileType} from "../edit-course";
import {UploadFileListener} from "../global";

const dateFormatOptions: Intl.DateTimeFormatOptions = {
    weekday: "long",
    year: "numeric",
    month: "short",
    day: "2-digit",
};
const timeFormatOptions: Intl.DateTimeFormatOptions = {
    hour: "2-digit",
    minute: "2-digit",
};

export class AdminLectureListProvider extends StreamableMapProvider<number, Lecture[]> {
    protected async fetcher(courseId: number): Promise<Lecture[]> {
        const result = await AdminLectureList.get(courseId);
        return result.map((s) => {
            s.hasAttachments = (s.files || []).some((f) => f.fileType === FileType.attachment);

            s.startDate = new Date(s.start);
            s.startDateFormatted = s.startDate.toLocaleDateString("en-US", dateFormatOptions);
            s.startTimeFormatted = s.startDate.toLocaleTimeString("en-US", timeFormatOptions);

            s.endDate = new Date(s.end);
            s.endDateFormatted = s.endDate.toLocaleDateString("en-US", dateFormatOptions);
            s.endTimeFormatted = s.endDate.toLocaleTimeString("en-US", timeFormatOptions);

            s.newCombinedVideo = null;
            s.newPresentationVideo = null;
            s.newCameraVideo = null;

            return s;
        });
    }

    async add(courseId: number, lecture: Lecture): Promise<void> {
        await AdminLectureList.add(courseId, lecture);
        await this.fetch(courseId, true);
        await this.triggerUpdate(courseId);
    }

    async setPrivate(courseId: number, lectureId: number, isPrivate: boolean) {
        await AdminLectureList.setPrivate(lectureId, isPrivate);
        this.data[courseId] = (await this.getData(courseId)).map((s) => {
            if (s.lectureId !== lectureId) {
                return s;
            }
            return {
                ...s,
                private: !s.private
            }
        });
        await this.triggerUpdate(courseId);
    }

    async delete(courseId: number, lectureIds: number[]) {
        await AdminLectureList.delete(courseId, lectureIds);
        this.data[courseId] = (await this.getData(courseId)).filter((s) => !lectureIds.includes(s.lectureId));
        await this.triggerUpdate(courseId);
    }

    async updateMeta(courseId: number, lectureId: number, request: UpdateLectureMetaRequest) {
        await AdminLectureList.update(courseId, lectureId, request);

        this.data[courseId] = (await this.getData(courseId)).map((s) => {
            if (s.lectureId === lectureId) {
                s = {
                    ...s,
                };

                // Patch updated keys in local data
                for (const requestKey in request) {
                    const val = request[requestKey];
                    if (val !== undefined) {
                        s[requestKey] = val;
                    }
                }
            }
            return s;
        });
        await this.triggerUpdate(courseId);
    }

    async uploadVideo(courseId: number, lectureId: number, videoType: string, file: File, listener : UploadFileListener = {}) {
        await AdminLectureList.uploadVideo(courseId, lectureId, videoType, file, listener);
    }
}
