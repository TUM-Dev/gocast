import {StreamableMapProvider} from "./provider";
import {Subtitle, SubtitleAPI} from "../api/subtitles";

export class SubtitleProvider extends StreamableMapProvider<number, Subtitle> {
    protected async fetcher(streamId: number): Promise<Subtitle> {
        let sub = await SubtitleAPI.get(streamId)
        console.log(sub);
        return sub;
    }
}
