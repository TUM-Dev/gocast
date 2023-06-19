import { get, post } from "../utilities/fetch-wrappers";

export class User implements Identifiable {
    ID: number;
    name: string;
    isAdmin: boolean;
}

export type HasPinnedCourseDTO = {
    has: boolean;
};

/**
 * REST API Wrapper for /api/users
 */
export const UserAPI = {
    async hasPinnedCourse(courseID: number) {
        return get(`/api/users/courses/${courseID}/pin`, { has: false });
    },

    async pinCourse(courseID: number) {
        return post("/api/users/courses/pin", { courseID });
    },

    async unpinCourse(courseID: number) {
        return post("/api/users/courses/unpin", { courseID });
    },
};
