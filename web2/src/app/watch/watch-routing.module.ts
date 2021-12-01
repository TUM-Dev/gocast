import {NgModule} from '@angular/core';
import {Routes, RouterModule} from '@angular/router'; // CLI imports router
import {WatchComponent} from './watch.component';

const routes: Routes = [
  {
    path: ':slug/:id',
    component: WatchComponent,
  },
];

@NgModule({
  imports: [RouterModule.forChild(routes)],
  exports: [RouterModule]
})
export class WatchRoutingModule {
}
