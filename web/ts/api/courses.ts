import { get } from "../utilities/fetch-wrappers";
import { Progress } from "./progress";
import { ToggleableElement } from "../utilities/ToggleableElement";
import { same_day } from "../utilities/time-utils";

type DownloadableVOD = {
    readonly FriendlyName: string;
    readonly DownloadURL: string;
};

export class Stream {
    readonly ID: number;
    readonly Name: string;
    readonly IsRecording: boolean;
    readonly IsPlanned: boolean;
    readonly IsComingUp: boolean;
    readonly Description: string;
    readonly HLSUrl: string;
    readonly End: string;
    readonly Start: string;
    readonly Downloads: DownloadableVOD[];

    Progress?: Progress;

    Dropdown = new ToggleableElement([["downloads", new ToggleableElement()]]);
    Thumbnail?: HTMLImageElement;

    public HasName(): boolean {
        return this.Name !== "";
    }

    public FriendlyDateStart(): string {
        return new Date(this.Start).toLocaleString();
    }

    public MonthOfStart(): string {
        return new Date(this.Start).toLocaleString("default", { month: "short" });
    }

    public DayOfStart(): number {
        return new Date(this.Start).getDate();
    }

    public TimeOfStart(): string {
        return Stream.TimeOf(this.Start);
    }

    public TimeOfEnd(): string {
        return Stream.TimeOf(this.End);
    }

    public IsToday(): boolean {
        return same_day(new Date(this.Start), new Date());
    }

    public MinutesLeftToStart(): number {
        return Math.round((new Date(this.Start).valueOf() - new Date().valueOf()) / 60000);
    }

    public UntilString(): string {
        const end = new Date(this.End);
        const hours = end.getHours();
        const minutes = end.getMinutes();
        return `Until ${hours}:${minutes < 10 ? minutes + "0" : minutes}`;
    }

    public HasDownloads(): boolean {
        return this.Downloads.length > 0;
    }

    public CompareStart(other: Stream) {
        const a = new Date(this.Start);
        const b = new Date(other.Start);
        if (a < b) {
            return 1;
        } else if (a > b) {
            return -1;
        }
        return 0;
    }

    public FetchThumbnail() {
        this.Thumbnail = new Image();
        this.Thumbnail.src = `/api/stream/${this.ID}/thumbs/vod`;
    }

    private static TimeOf(d: string): string {
        return new Date(d).toLocaleTimeString("default", { hour: "2-digit", minute: "2-digit" });
    }
}

export class Course {
    readonly ID: number;
    readonly Visibility: string;
    readonly Slug: string;
    readonly Year: number;
    readonly TeachingTerm: string;
    readonly Name: string;

    readonly DownloadsEnabled: boolean;

    readonly NextLecture?: Stream;
    readonly LastRecording?: Stream;

    readonly Pinned: boolean = false;

    private readonly Streams?: Stream[];

    readonly Recordings?: Stream[];
    readonly Planned?: Stream[];
    readonly Upcoming?: Stream[];

    static New(obj): Course {
        const c = Object.assign(new Course(), obj);
        c.NextLecture = obj.NextLecture ? Object.assign(new Stream(), obj.NextLecture) : undefined;
        c.LastRecording = obj.LastRecording ? Object.assign(new Stream(), obj.LastRecording) : undefined;
        c.Streams = obj.Streams ? obj.Streams.map((s) => Object.assign(new Stream(), s)) : [];
        c.Recordings = c.Streams.filter((s) => s.IsRecording);
        c.Planned = c.Streams.filter((s) => s.IsPlanned);
        c.Upcoming = c.Streams.filter((s) => s.IsComingUp);
        return c;
    }

    public URL(): string {
        return `?year=${this.Year}&term=${this.TeachingTerm}&slug=${this.Slug}&view=3`;
    }

    public LastRecordingURL(): string {
        return this.WatchURL(this.LastRecording.ID);
    }

    public NextLectureURL(): string {
        return this.WatchURL(this.NextLecture.ID);
    }

    public ICS(): string {
        return `/api/download_ics/${this.Year}/${this.TeachingTerm}/${this.Slug}/events.ics`;
    }

    public WatchURL(id: number): string {
        return `/w/${this.Slug}/${id}`;
    }

    public IsHidden(): boolean {
        return this.Visibility === "hidden";
    }
}

export class LectureHall {
    readonly Name: string;
    readonly ExternalURL: string;

    constructor(obj: LectureHall) {
        this.Name = obj.Name;
        this.ExternalURL = obj.ExternalURL;
    }
}

export class Livestream {
    readonly Stream: Stream;
    readonly Course: Course;
    readonly LectureHall?: LectureHall;
    readonly Viewers: number;

    constructor(obj: Livestream) {
        this.Stream = Object.assign(new Stream(), obj.Stream);
        this.Course = Course.New(obj.Course);
        this.LectureHall = obj.LectureHall ? new LectureHall(obj.LectureHall) : undefined;
        this.Viewers = obj.Viewers;
    }

    public InLectureHall(): boolean {
        return this.LectureHall !== undefined;
    }
}

/**
 * REST API Wrapper for /api/courses
 */
export const CoursesAPI = {
    async getLivestreams() {
        return get("/api/courses/live").then((livestreams) => livestreams.map((l) => new Livestream(l)));
    },

    async getPublic(year?: number, term?: string): Promise<object> {
        return get(`/api/courses/public${this.query(year, term)}`).then((courses) => courses.map((c) => Course.New(c)));
    },

    async getUsers(year?: number, term?: string): Promise<object> {
        return get(`/api/courses/users${this.query(year, term)}`).then((courses) => courses.map((c) => Course.New(c)));
    },

    async getPinned(year?: number, term?: string): Promise<object> {
        return get(`/api/courses/users/pinned${this.query(year, term)}`).then((courses) =>
            courses.map((c) => Course.New(c)),
        );
    },

    async get(slug: string, year?: number, term?: string) {
        return get(`/api/courses/${slug}${this.query(year, term)}`, {}, true).then((course) => Course.New(course));
    },

    query: (year?: number, term?: string) =>
        year !== undefined && term !== undefined && year !== 0 ? `?year=${year}&term=${term}` : "",
};
