import { get } from "../utilities/fetch-wrappers";
import { Progress } from "./progress";

export class Stream {
    readonly ID: number;
    readonly Name: string;
    readonly End: string;
    readonly Start: string;

    Progress?: Progress;

    public HasName(): boolean {
        return this.Name !== "";
    }

    public FriendlyDateStart(): string {
        return new Date(this.Start).toLocaleString();
    }

    public UntilString(): string {
        const end = new Date(this.End);
        const hours = end.getHours();
        const minutes = end.getMinutes();
        return `Until ${hours}:${minutes < 10 ? minutes + "0" : minutes}`;
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
    readonly LastRecording?: Stream;

    static New(obj): Course {
        const c = Object.assign(new Course(), obj);
        c.NextLecture = obj.NextLecture ? Object.assign(new Stream(), obj.NextLecture) : undefined;
        c.LastRecording = obj.LastRecording ? Object.assign(new Stream(), obj.LastRecording) : undefined;
        return c;
    }

    public URL(): string {
        return `/course/${this.Year}/${this.TeachingTerm}/${this.Slug}`;
    }

    public LastRecordingURL(): string {
        return `/w/${this.Slug}/${this.LastRecording.ID}`;
    }

    public NextLectureURL(): string {
        return `/w/${this.Slug}/${this.NextLecture.ID}`;
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

export class Stat {
    readonly Viewers: number;
}

export class Livestream {
    readonly Stream: Stream;
    readonly Course: Course;
    readonly LectureHall?: LectureHall;
    readonly Stat: Stat;

    constructor(obj: Livestream) {
        this.Stream = Object.assign(new Stream(), obj.Stream);
        this.Course = Course.New(obj.Course);
        this.LectureHall = obj.LectureHall ? new LectureHall(obj.LectureHall) : undefined;
        this.Stat = Object.assign(new Stat(), obj.Stat);
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

    query: (year?: number, term?: string) =>
        year !== undefined && term !== undefined && year !== 0 ? `?year=${year}&term=${term}` : "",
};
