/**
 * Time Utility Class
 * Conversion of seconds to (hours, minutes, seconds) and vice versa.
 */
export class Time {
    private readonly hours: number;
    private readonly minutes: number;
    private readonly seconds: number;

    static FromSeconds(seconds: number): Time {
        const date = new Date(seconds * 1000);
        return new Time(date.getUTCHours(), date.getUTCMinutes(), date.getUTCSeconds());
    }

    constructor(hours = 0, minutes = 0, seconds = 0) {
        this.hours = hours;
        this.minutes = minutes;
        this.seconds = seconds;
    }

    public toString() {
        let s = `${Time.padZero(this.minutes)}:${Time.padZero(this.seconds)}`;
        if (this.hours > 0) {
            s = `${Time.padZero(this.hours)}:` + s;
        }
        return s;
    }

    public toStringWithLeadingZeros() {
        return `${Time.padZero(this.hours)}:${Time.padZero(this.minutes)}:${Time.padZero(this.seconds)}`;
    }

    public toSeconds(): number {
        return this.hours * 60 * 60 + this.minutes * 60 + this.seconds;
    }

    public toObject() {
        return { hours: this.hours, minutes: this.minutes, seconds: this.seconds };
    }

    private static padZero(i: string | number) {
        if (typeof i === "string") {
            i = parseInt(i);
        }
        if (i < 10) {
            i = "0" + i;
        }
        return i;
    }
}
