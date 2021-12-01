import { Component, OnInit } from '@angular/core';
import {Title} from "@angular/platform-browser";

@Component({
  selector: 'app-overview',
  templateUrl: './overview.component.html',
  styleUrls: ['./overview.component.css']
})
export class OverviewComponent implements OnInit {

  constructor(private titleService:Title) {
    titleService.setTitle('TUM-Live | Admin')
  }

  ngOnInit(): void {
  }

}
