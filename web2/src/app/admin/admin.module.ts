import { NgModule } from '@angular/core';
import { CommonModule } from '@angular/common';
import { OverviewComponent } from './overview/overview.component';
import {AdminRoutingModule} from "./admin-routing.module";
import { CourseComponent } from './course/course.component';

@NgModule({
  declarations: [
    OverviewComponent,
    CourseComponent
  ],
  imports: [
    CommonModule,
    AdminRoutingModule,
  ],
  bootstrap: [
    OverviewComponent,
  ]
})
export class AdminModule { }
