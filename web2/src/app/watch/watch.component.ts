import {Component, OnInit} from '@angular/core';
import Stream from "../model/stream.model";
import Course from "../model/course.model";
import {ActivatedRoute} from "@angular/router";
import {WatchService} from "./watch.service";

@Component({
  selector: 'app-watch',
  templateUrl: './watch.component.html',
  styleUrls: ['./watch.component.less']
})
export class WatchComponent implements OnInit {
  public stream: Stream | undefined;
  public course: Course | undefined;
  public chatEnabled: boolean | undefined;
  public loading: boolean = true;
  public theaterMode: boolean = false;

  constructor(private route: ActivatedRoute, private watchService: WatchService) {
    route.paramMap.subscribe(value => {
      watchService.getWatchPage(value.get('id')).subscribe(value => {
        this.stream = value.stream;
        this.course = value.course;
        this.chatEnabled = value.course.chatEnabled;
        this.loading = false;
      });
    });
  }

  ngOnInit(): void {

  }


  getPlaylistURL(): string {
    return this.stream ? this.stream.playlists[0].url : '';
  }
}
