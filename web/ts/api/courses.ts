import { get } from "../utilities/fetch-wrappers";
import { Progress } from "./progress";

const DEFAULT_LECTURE_NAME = "Untitled lecture";

export class Stream {
    readonly ID: number;
    readonly Name: string;
    readonly IsRecording: boolean;
    readonly IsPlanned: boolean;
    readonly Description: string;
    readonly End: string;
    readonly Start: string;

    Progress?: Progress;

    static New(obj): Stream {
        const s = Object.assign(new Stream(), obj);
        s.Name = s.Name === "" ? DEFAULT_LECTURE_NAME : s.Name;
        return s;
    }

    public FriendlyDateStart(): string {
        return new Date(this.Start).toLocaleString();
    }

    public MonthOfStart(): string {
        return new Date(this.Start).toLocaleString("default", { month: "long" });
    }

    public DayOfStart(): number {
        return new Date(this.Start).getDate();
    }

    public TimeOfStart(): string {
        const s = new Date(this.Start);
        const hours = s.getUTCHours().toString().padStart(2, "0");
        const minutes = s.getUTCMinutes().toString().padStart(2, "0");
        return `${hours}:${minutes}`;
    }

    public UntilString(): string {
        const end = new Date(this.End);
        const hours = end.getHours();
        const minutes = end.getMinutes();
        return `Until ${hours}:${minutes < 10 ? minutes + "0" : minutes}`;
    }

    public CompareStart(other: Stream) {
        const a = new Date(this.Start);
        const b = new Date(other.Start);
        if (a < b) {
            return 1;
        } else if (b > a) {
            return -1;
        }
        return 0;
    }
}

export class Course {
    readonly ID: number;
    readonly Visibility: string;
    readonly Slug: string;
    readonly Year: number;
    readonly TeachingTerm: string;
    readonly Name: string;

    readonly NextLecture?: Stream;
    readonly LastLecture?: Stream;

    readonly Pinned: boolean = false;

    private readonly Streams?: Stream[];

    readonly Recordings?: Stream[];
    readonly Planned?: Stream[];

    static New(obj): Course {
        const c = Object.assign(new Course(), obj);
        c.NextLecture = obj.NextLecture ? Stream.New(obj.NextLecture) : undefined;
        c.LastLecture = obj.LastLecture ? Stream.New(obj.LastLecture) : undefined;
        c.Streams = obj.Streams ? obj.Streams.map((s) => Stream.New(s)) : [];
        c.Recordings = c.Streams.filter((s) => s.IsRecording);
        c.Planned = c.Streams.filter((s) => s.IsPlanned);
        return c;
    }

    public URL(): string {
        return `/course/${this.Year}/${this.TeachingTerm}/${this.Slug}`;
    }

    public LastLectureURL(): string {
        return this.WatchURL(this.LastLecture.ID);
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

    constructor(obj: Livestream) {
        this.Stream = Stream.New(obj.Stream);
        this.Course = Course.New(obj.Course);
        this.LectureHall = obj.LectureHall ? new LectureHall(obj.LectureHall) : undefined;
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

    async get(slug: string, year?: number, term?: string) {
        return get(`/api/courses/${slug}${this.query(year, term)}`, {}, true).then((course) => Course.New(course));
    },

    query: (year?: number, term?: string) =>
        year !== undefined && term !== undefined && year !== 0 ? `?year=${year}&term=${term}` : "",
};
