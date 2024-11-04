import { StreamableMapProvider } from "./provider";

export class TranscriptProvider extends StreamableMapProvider<number, string[]> {
    protected async fetcher(): Promise<string[]> {
        return [];
    }
}

