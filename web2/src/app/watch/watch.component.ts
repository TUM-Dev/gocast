import {Component, OnInit} from '@angular/core';
import Stream from "../model/stream.model";
import Course from "../model/course.model";
import {ActivatedRoute} from "@angular/router";
import {WatchService} from "./watch.service";
import WatchDto from "../dto/watch.dto";

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

  constructor(private route: ActivatedRoute, private watchService: WatchService) {
    let w: WatchDto = {
      stream:{
      name: "Vorlesung 3",
        start: new Date(),
        end: new Date(),
        id: 1,
        description: "tweedback irgendwo.",
        Chats: [],
    },
      course: {
        Id: 1,
        ChatEnabled: true,
        Slug: "test",
        Streams: [],
        Title: "Einführung in die Informatik",
      }
    }
    console.log(JSON.stringify(w));
    route.paramMap.subscribe(value => {
      watchService.getWatchPage(value.get('id')).subscribe(value => {
        this.stream = value.stream;
        this.course = value.course;
        this.chatEnabled = value.course.ChatEnabled;
        this.loading = false;
      });
    });
  }

  ngOnInit(): void {

  }


}
