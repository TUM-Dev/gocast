import {Component, ElementRef, Input, OnDestroy, OnInit, ViewChild, ViewEncapsulation} from '@angular/core';
import videojs from 'video.js';
import {debounceTime, fromEvent, map, startWith} from "rxjs";
import {theaterMode, TheaterModeToggle} from './TUMLiveVjs'
import {WatchComponent} from "../watch.component";

function windowSizeObserver(dTime = 300) {
  return fromEvent(window, 'resize').pipe(
    debounceTime(dTime),
    map(event => {
      const window = event.target as Window;

      return {width: window.innerWidth, height: window.innerHeight}
    }),
    startWith({width: window.innerWidth, height: window.innerHeight})
  );
}

@Component({
  selector: 'app-player',
  template: `
    <div #wrap>
      <video #target class="video-js" controls preload="none"></video>
    </div>
  `,
  styleUrls: ['./player.component.less'],
  encapsulation: ViewEncapsulation.None,
})
export class PlayerComponent implements OnInit, OnDestroy {
  @ViewChild('target', {static: true}) target: ElementRef | undefined;
  @ViewChild('wrap', {static: true}) wrap: ElementRef | undefined;

  // see options: https://github.com/videojs/video.js/blob/maintutorial-options.html
  @Input() options: {
    fill: boolean;
    aspectRatio: string;
    liveui: boolean;
    autoplay: boolean;
    controls: boolean;
    sources: {
      src: string;
      type: string;
    }[];
  } | undefined;
  player: videojs.Player | undefined;
  @Input() caller: WatchComponent | undefined;

  constructor(
    private elementRef: ElementRef,
  ) {
    windowSizeObserver().subscribe(size => {
      console.log(size);
    });
  }

  ngOnInit() {
    // instantiate Video.js
    videojs.registerComponent('theaterModeToggle', TheaterModeToggle);
    videojs.registerPlugin('theaterMode', theaterMode);
    this.player = videojs(this.target!.nativeElement, this.options, function onPlayerReady() {
      console.log('onPlayerReady', this);
    });

    // @ts-ignore
    this.player.theaterMode({elementToToggle: 'my-video', className: 'theater-mode', caller: this});
  }

  toggleTheaterMode() {
    this.caller!.theaterMode = !this.caller!.theaterMode;
    this.player?.fluid(this.caller!.theaterMode);
    console.log(this.caller!.theaterMode);
  }

  ngOnDestroy() {
    // destroy player
    if (this.player) {
      this.player.dispose();
    }
  }
}
