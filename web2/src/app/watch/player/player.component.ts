import {Component, ElementRef, Input, OnDestroy, OnInit, ViewChild, ViewEncapsulation} from '@angular/core';
import videojs from 'video.js';
import {debounceTime, fromEvent, map, startWith} from "rxjs";

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

  constructor(
    private elementRef: ElementRef,
  ) {
    windowSizeObserver().subscribe(size => {
      console.log(size);
      //this.player?.height(size.height);
      this.wrap? this.wrap.nativeElement.style.maxHeight = `calc(7rem - 5rem)`: null;
    });
  }

  ngOnInit() {
    // instantiate Video.js
    this.player = videojs(this.target!.nativeElement, this.options, function onPlayerReady() {
      console.log('onPlayerReady', this);
    });
  }

  ngOnDestroy() {
    // destroy player
    if (this.player) {
      this.player.dispose();
    }
  }
}
