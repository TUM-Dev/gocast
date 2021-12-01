import {NgModule} from '@angular/core';
import {Routes, RouterModule} from '@angular/router'; // CLI imports router
import {CourseComponent} from "./course/course.component";
import {OverviewComponent} from "./overview/overview.component";

const routes: Routes = [
  {
    path: '',
    component: OverviewComponent,
  },
  {
    path: 'course',
    component: CourseComponent,
  },
];

// configures NgModule imports and exports
@NgModule({
  imports: [RouterModule.forChild(routes)],
  exports: [RouterModule]
})
export class AdminRoutingModule {
}
