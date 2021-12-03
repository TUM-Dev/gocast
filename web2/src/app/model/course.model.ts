import Stream from "./stream.model";

export default class Course {

  constructor(
    public id: number,
    public slug: string,
    public chatEnabled: boolean,
    public title: string,
    public streams: Stream[],
  ) {
  }

}
