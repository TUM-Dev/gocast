import { get } from "../utilities/fetch-wrappers";

export type SemesterDTO = {
    Current: Semester;
    Semesters: Semester[];
};

export class Semester {
    TeachingTerm: string;
    Year: number;

    constructor(obj: Semester) {
        this.TeachingTerm = obj.TeachingTerm;
        this.Year = obj.Year;
    }

    public FriendlyString(): string {
        if (this.TeachingTerm === "W") return `Winter ${this.Year}/${this.Year + 1}`;
        else return `Summer ${this.Year}`;
    }
}

/**
 * REST API Wrapper for /api/semesters
 */
export const SemestersAPI = {
    async get(): Promise<SemesterDTO> {
        return get("/api/semesters").then((l: SemesterDTO) => {
            l.Semesters = l.Semesters.map((s) => new Semester(s));
            return l;
        });
    },
};
