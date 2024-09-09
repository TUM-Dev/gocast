export function isLectureHallValid(lectureHall: string): boolean {
    const regex = /^\d{4}\.[A-Z0-9]{2}\.[A-Z0-9]{3,4}$/;
    return regex.test(lectureHall);
}

declare global {
    interface Window {
        isLectureHallValid: (lectureHall: string) => boolean;
    }
}

window.isLectureHallValid = isLectureHallValid;
