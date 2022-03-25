class User {
    id: number;
    name: string;
    login: string;
}

export function courseAdminManagement(): { m: CourseAdminManagement } {
    return { m: new CourseAdminManagement() };
}

export class CourseAdminManagement {
    users: User[] = [];
    searchResult: User[] = [];
    courseId: number;
    search = "";

    userId: number;

    init(courseId: number, userId: number) {
        this.courseId = courseId;
        this.userId = userId;

        fetch(`/api/course/${courseId}/admins`)
            .then((response) => response.json() as Promise<User[]>)
            .then((users) => {
                this.users = users;
            });
    }

    searchUsers() {
        if (this.search.length < 3) {
            this.searchResult = [];
            return;
        }
        fetch(`/api/searchUserForCourse?q=${this.search}`)
            .then((response) => response.json() as Promise<User[]>)
            .then((users) => {
                this.searchResult = users;
            });
    }

    addAdmin(id: number) {
        fetch(`/api/course/${this.courseId}/admins/${id}`, { method: "PUT" })
            .then((response) => response.json() as Promise<User>)
            .then((user) => {
                this.users.push(user);
                this.searchResult = this.searchResult.filter((u) => u.id !== user.id);
            });
    }

    removeAdmin(id: number) {
        if (id === this.userId) {
            if (!confirm("Are you sure you want to remove yourself from the course admins?")) {
                return;
            }
        }
        fetch(`/api/course/${this.courseId}/admins/${id}`, { method: "DELETE" })
            .then((response) => response.json() as Promise<User>)
            .then((user) => {
                if (id === this.userId) {
                    // user is no longer admin of the course, redirect them to the start page
                    window.location.href = "/";
                } else {
                    this.users = this.users.filter((u) => u.id !== user.id);
                }
            });
    }
}
