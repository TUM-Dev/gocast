import {NgModule} from '@angular/core';
import {BrowserModule} from '@angular/platform-browser';

import {AppComponent} from './app.component';
import {StartComponent} from './start/start.component';
import {AppRoutingModule} from "./app-routing.module";
import { FontAwesomeModule } from '@fortawesome/angular-fontawesome';

// Lazy load admin modules!
@NgModule({
  declarations: [
    AppComponent,
    StartComponent,
  ],
  imports: [
    BrowserModule,
    AppRoutingModule,
    FontAwesomeModule
  ],
  providers: [],
  bootstrap: [AppComponent]
})
export class AppModule {
}
