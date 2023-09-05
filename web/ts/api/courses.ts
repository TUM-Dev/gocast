import { get } from "../utilities/fetch-wrappers";
import { Progress } from "./progress";
import { ToggleableElement } from "../utilities/ToggleableElement";
import { same_day } from "../utilities/time-utils";
import { CustomURL } from "../utilities/url";

type DownloadableVOD = {
    readonly FriendlyName: string;
    readonly DownloadURL: string;
};

export class Stream implements Identifiable {
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
    readonly Duration: number;

    Progress?: Progress;

    Dropdown = new ToggleableElement([["downloads", new ToggleableElement()]]);
    Thumbnail?: HTMLImageElement;

    public HasName(): boolean {
        return this.Name !== "";
    }

    public FriendlyDateStart(): string {
        return new Date(this.Start).toLocaleString("default", {
            weekday: "long",
            year: "numeric",
            month: "numeric",
            day: "numeric",
            hour: "2-digit",
            minute: "2-digit",
        });
    }

    public StartDate(): Date {
        return new Date(this.Start);
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

    public DurationString() {
        return new Date(this.Duration * 1000).toISOString().slice(11, 19);
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

    public GetMonthName(): string {
        return [
            "January",
            "February",
            "March",
            "April",
            "May",
            "June",
            "July",
            "August",
            "September",
            "October",
            "November",
            "December",
        ][this.StartDate().getMonth()];
    }

    private static TimeOf(d: string): string {
        return new Date(d).toLocaleTimeString("default", { hour: "2-digit", minute: "2-digit" });
    }
}

export class Course implements Identifiable {
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

    readonly IsAdmin: boolean;

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

    static Compare(a: Course, b: Course): number {
        return a.Name.localeCompare(b.Name);
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

    async getPublic(year?: number, term?: string): Promise<Course[]> {
        const url = new CustomURL("/api/courses/public", { year, term });
        return get(url.toString()).then((courses) => courses.map((c) => Course.New(c)));
    },

    async getUsers(year?: number, term?: string): Promise<Course[]> {
        const url = new CustomURL("/api/courses/users", { year, term });
        return get(url.toString()).then((courses) => courses.map((c) => Course.New(c)));
    },

    async getPinned(year?: number, term?: string): Promise<Course[]> {
        const url = new CustomURL("/api/courses/users/pinned", { year, term });
        return get(url.toString()).then((courses) => courses.map((c) => Course.New(c)));
    },

    async get(slug: string, year?: number, term?: string, userId?: number) {
        const url = new CustomURL(`/api/courses/${slug}`, { year, term, userId });
        return get(url.toString(), {}, true).then((course) => Course.New(course));
    },
};
