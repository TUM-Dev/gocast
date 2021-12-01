import { NgModule } from '@angular/core';
import { CommonModule } from '@angular/common';
import { WatchComponent } from './watch.component';
import { AdminComponent } from './admin/admin.component';
import { ChatComponent } from './chat/chat.component';
import {WatchRoutingModule} from "./watch-routing.module";
import { PlayerComponent } from './player/player.component';
import {HttpClientModule} from "@angular/common/http";

@NgModule({
  declarations: [
    WatchComponent,
    AdminComponent,
    ChatComponent,
    PlayerComponent,
  ],
  imports: [
    CommonModule,
    WatchRoutingModule,
    HttpClientModule,
  ],
  bootstrap: [WatchComponent],
})
export class WatchModule {
}
