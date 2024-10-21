import { StreamableMapProvider } from "./provider";
import {
    AdminLectureList,
    Lecture,
    LectureFile,
    UpdateLectureMetaRequest,
    UpdateLectureStartEndRequest,
    VideoSection,
    videoSectionSort,
} from "../api/admin-lecture-list";
import { FileType } from "../edit-course";
import { PostFormDataListener } from "../utilities/fetch-wrappers";

export const dateFormatOptions: Intl.DateTimeFormatOptions = {
    weekday: "long",
    year: "numeric",
    month: "short",
    day: "2-digit",
};
export const timeFormatOptions: Intl.DateTimeFormatOptions = {
    hour: "2-digit",
    minute: "2-digit",
};

export interface UpdateMetaProps {
    payload: UpdateLectureMetaRequest;
    options?: {
        saveSeries?: boolean;
    };
}

export class AdminLectureListProvider extends StreamableMapProvider<number, Lecture[]> {
    protected async fetcher(courseId: number): Promise<Lecture[]> {
        const result = await AdminLectureList.get(courseId);
        return result.map((s) => {
            s.hasAttachments = (s.files || []).some((f) => f.fileType === FileType.attachment);

            s.videoSections = (s.videoSections ?? []).sort(videoSectionSort);

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
                private: !s.private,
            };
        });
        await this.triggerUpdate(courseId);
    }

    async delete(courseId: number, lectureIds: number[]) {
        await AdminLectureList.delete(courseId, lectureIds);
        this.data[courseId] = (await this.getData(courseId)).filter((s) => !lectureIds.includes(s.lectureId));
        await this.triggerUpdate(courseId);
    }

    async deleteSeries(courseId: number, lectureId: number) {
        await AdminLectureList.deleteSeries(courseId, lectureId);

        const lectures = await this.getData(courseId);
        const seriesIdentifier = lectures.find((l) => l.lectureId === lectureId)?.seriesIdentifier ?? null;
        const lectureIds = lectures.filter((l) => l.seriesIdentifier === seriesIdentifier).map((l) => l.lectureId);

        this.data[courseId] = lectures.filter((s) => !lectureIds.includes(s.lectureId));
        await this.triggerUpdate(courseId);
    }

    async updateMeta(courseId: number, lectureId: number, props: UpdateMetaProps) {
        const updateSeries = props?.options?.saveSeries === true;
        const seriesIdentifier =
            (await this.getData(courseId)).find((l) => l.lectureId === lectureId)?.seriesIdentifier ?? null;

        await AdminLectureList.updateMetadata(courseId, lectureId, props.payload);
        if (updateSeries) {
            await AdminLectureList.saveSeriesMetadata(courseId, lectureId);
        }

        this.data[courseId] = (await this.getData(courseId)).map((s) => {
            const isLecture = s.lectureId === lectureId;
            const isInLectureSeries = s.seriesIdentifier === seriesIdentifier;

            if (isLecture || (updateSeries && isInLectureSeries)) {
                s = {
                    ...s,
                };

                // Patch updated keys in local data
                for (const requestKey in props.payload) {
                    const val = props.payload[requestKey];
                    if (val !== undefined) {
                        s[requestKey] = val;
                    }
                }
            }
            return s;
        });
        await this.triggerUpdate(courseId);
    }

    async updateStartEnd(courseId: number, lectureId: number, payload: UpdateLectureStartEndRequest) {
        await AdminLectureList.updateStartEnd(courseId, lectureId, payload);

        this.data[courseId] = (await this.getData(courseId)).map((s) => {
            if (s.lectureId === lectureId) {
                return {
                    ...s,
                    start: payload.start,
                    end: payload.end,
                };
            }
            return s;
        });
        await this.triggerUpdate(courseId);
    }

    async uploadAttachmentFile(courseId: number, lectureId: number, file: File) {
        const res = await AdminLectureList.uploadAttachmentFile(courseId, lectureId, file);
        const newFile = new LectureFile({
            id: JSON.parse(res.responseText),
            fileType: 2,
            friendlyName: file.name,
        });

        this.data[courseId] = (await this.getData(courseId)).map((s) => {
            if (s.lectureId === lectureId) {
                return {
                    ...s,
                    files: [...s.files, newFile],
                };
            }
            return s;
        });
        await this.triggerUpdate(courseId);
    }

    async uploadAttachmentUrl(courseId: number, lectureId: number, url: string) {
        const res = await AdminLectureList.uploadAttachmentUrl(courseId, lectureId, url);
        const newFile = new LectureFile({
            id: JSON.parse(res.responseText),
            fileType: 2,
            friendlyName: url.substring(url.lastIndexOf("/") + 1),
        });

        this.data[courseId] = (await this.getData(courseId)).map((s) => {
            if (s.lectureId === lectureId) {
                return {
                    ...s,
                    files: [...s.files, newFile],
                };
            }
            return s;
        });
        await this.triggerUpdate(courseId);
    }

    async deleteAttachment(courseId: number, lectureId: number, attachmentId: number) {
        await AdminLectureList.deleteAttachment(courseId, lectureId, attachmentId);

        this.data[courseId] = (await this.getData(courseId)).map((s) => {
            if (s.lectureId === lectureId) {
                return {
                    ...s,
                    files: [...s.files.filter((a) => a.id !== attachmentId)],
                };
            }
            return s;
        });
        await this.triggerUpdate(courseId);
    }

    async addSections(courseId: number, lectureId: number, videoSections: VideoSection[]) {
        const newSections = await AdminLectureList.addSections(lectureId, videoSections);

        this.data[courseId] = (await this.getData(courseId)).map((s) => {
            if (s.lectureId === lectureId) {
                return {
                    ...s,
                    videoSections: [...s.videoSections, ...newSections],
                };
            }
            return s;
        });
        await this.triggerUpdate(courseId);
    }

    async updateSection(courseId: number, lectureId: number, videoSection: VideoSection) {
        await AdminLectureList.updateSection(lectureId, videoSection);

        this.data[courseId] = (await this.getData(courseId)).map((s) => {
            if (s.lectureId === lectureId) {
                return {
                    ...s,
                    videoSections: [...s.videoSections.filter((a) => a.id !== videoSection.id), videoSection].sort(
                        videoSectionSort,
                    ),
                };
            }
            return s;
        });
        await this.triggerUpdate(courseId);
    }

    async deleteSection(courseId: number, lectureId: number, videoSectionId: number) {
        await AdminLectureList.deleteSection(lectureId, videoSectionId);

        this.data[courseId] = (await this.getData(courseId)).map((s) => {
            if (s.lectureId === lectureId) {
                return {
                    ...s,
                    videoSections: [...s.videoSections.filter((a) => a.id !== videoSectionId)].sort(videoSectionSort),
                };
            }
            return s;
        });
        await this.triggerUpdate(courseId);
    }

    async uploadVideo(
        courseId: number,
        lectureId: number,
        videoType: string,
        file: File,
        listener: PostFormDataListener = {},
    ) {
        await AdminLectureList.uploadVideo(courseId, lectureId, videoType, file, listener);
    }
}
