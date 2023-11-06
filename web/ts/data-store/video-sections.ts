import { Time } from "../global";
import { StreamableMapProvider } from "./provider";
import { Section, UpdateVideoSectionRequest, VideoSectionAPI } from "../api/video-sections";

export class VideoSectionProvider extends StreamableMapProvider<number, Section[]> {
    protected async fetcher(streamId: number): Promise<Section[]> {
        const result = await VideoSectionAPI.get(streamId);
        return result.map((s) => {
            s.friendlyTimestamp = new Time(s.startHours, s.startMinutes, s.startSeconds).toString();
            return s;
        });
    }

    async add(streamId: number, sections: Section[]): Promise<void> {
        await VideoSectionAPI.add(streamId, sections);
        await this.fetch(streamId, true);
        await this.triggerUpdate(streamId);
    }

    async delete(streamId: number, sectionId: number) {
        await VideoSectionAPI.delete(streamId, sectionId);
        this.data[streamId] = (await this.getData(streamId)).filter((s) => s.ID !== sectionId);
    }

    async update(streamId: number, sectionId: number, request: UpdateVideoSectionRequest) {
        await VideoSectionAPI.update(streamId, sectionId, request);
        this.data[streamId] = (await this.getData(streamId)).map((s) => {
            if (s.ID === sectionId) {
                s = {
                    ...s,
                    startHours: request.StartHours,
                    startMinutes: request.StartMinutes,
                    startSeconds: request.StartSeconds,
                    description: request.Description,
                    friendlyTimestamp: new Time(
                        request.StartHours,
                        request.StartMinutes,
                        request.StartSeconds,
                    ).toString(),
                };
            }
            return s;
        });
        await this.triggerUpdate(streamId);
    }
}
