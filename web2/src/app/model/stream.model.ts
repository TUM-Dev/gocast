export class Playlist {
  constructor(
    public type: string,
    public url: string,
  ) {
  }
}

export default class Stream {

  constructor(
    public id: number,
    public name: string,
    public start: Date,
    public end: Date,
    public description: string,
    public playlists: Playlist[],
  ) {
  }
}

