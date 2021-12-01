import Chat from "./chat.model";

export default class Stream {

  constructor(
    public id: number,
    public name: string,
    public start: Date,
    public end: Date,
    public description: string,
    public Chats: Chat[]
  ) {
  }
}

