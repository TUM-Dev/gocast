import Stream from "./stream.model";

export default class Course {

  constructor(
    public Id: number,
    public Slug: string,
    public ChatEnabled: boolean,
    public Title: string,
    public Streams: Stream[],
  ) {
  }

}
