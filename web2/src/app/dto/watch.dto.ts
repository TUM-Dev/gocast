import Stream from "../model/stream.model";
import Course from "../model/course.model";

export default class WatchDto {
  constructor(
    public stream: Stream,
    public course: Course,
  ) {
  }
}
