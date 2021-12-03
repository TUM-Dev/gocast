import videojs, {VideoJsPlayer} from "video.js";
import {PlayerComponent} from "./player.component";

const Button = videojs.getComponent('Button');

let skipTo = 0;

/**
 * Button to add a class to passed in element that will toggle "theater mode" as defined
 * in app's CSS (larger player, dimmed background, etc...)
 */

class SkipSilenceToggle extends Button {

  constructor(player: VideoJsPlayer, options: any) {
    super(player, options);
    this.controlText('Skip pause');
    // @ts-ignore // todo: fix this
    this.el().firstChild!.classList.add("icon-forward")
  }

  override buildCSSClass() {
    return `vjs-skip-silence-control ${super.buildCSSClass()}`;
  }

  override handleClick() {
    this.player().currentTime(skipTo);
  }
}

export class TheaterModeToggle extends Button {
  private caller: PlayerComponent;

  constructor(player: VideoJsPlayer, options: any) {
    super(player, options);
    console.log(options);
    this.caller = options.caller;
    this.controlText('Big picture mode');
    // @ts-ignore // todo: fix this
    this.el().firstChild!.classList.add("vjs-icon-theater-toggle");
  }

  override buildCSSClass() {
    return `vjs-theater-mode-control ${super.buildCSSClass()}`;
  }

  override handleClick() {
    this.caller.toggleTheaterMode();
    // @ts-ignore todo: fix this
    const theaterModeIsOn = document.getElementById(this.options_!.elementToToggle).classList.toggle(this.options_.className);
    this.player().trigger('theaterMode', {'theaterModeIsOn': theaterModeIsOn});
    if (theaterModeIsOn) {
      this.player().fluid(false);
    } else {
      this.player().fluid(true);
    }
  }
}


/**
 * @function theaterMode
 * @param    {Object} [options={}]
 *           elementToToggle, the name of the DOM element to add/remove the 'theater-mode' CSS class
 */
export const theaterMode = function (this: VideoJsPlayer, options: any) {
  this.ready(() => {
    this.addClass('vjs-theater-mode');
    let toggle = this.controlBar.addChild("theaterModeToggle", options);
    // @ts-ignore todo
    this.controlBar.el().insertBefore(toggle.el(), this.controlBar.fullscreenToggle.el());
  });

  this.on('fullscreenchange', () => {
    if (this.isFullscreen()) {
      this.controlBar.getChild("theaterModeToggle")!.hide();
    } else {
      this.controlBar.getChild("theaterModeToggle")!.show();
    }
  });
};

const skipSilence = function (this: VideoJsPlayer, options: any) {
  this.ready(() => {
    // @ts-ignore
    this.addClass('vjs-skip-silence');
    let toggle = this.addChild("SkipSilenceToggle");
    toggle.el().classList.add("invisible")
    this.el().insertBefore(toggle.el(), this.bigPlayButton.el());

    let isShowing = false;
    const silences = JSON.parse(options)
    const len = silences.length
    let i = 0;
    let n = 0;
    this.on('timeupdate', () => {
      if (n++ % 5 != 0) {
        return;
      }
      n = 0;
      const ctime = this.currentTime();
      let shouldShow = false;
      for (i = 0; i < len; i++) {
        if (ctime >= silences[i].start && ctime < silences[i].end) {
          shouldShow = true;
          skipTo = silences[i].end
          break
        }
      }
      if (isShowing && !shouldShow) {
        isShowing = false;
        toggle.el().classList.add("invisible");
      } else if (!isShowing && shouldShow) {
        isShowing = true;
        toggle.el().classList.remove("invisible");
      }
    });
  });
};

/**
 * @function watchProgress
 * Saves and retrieves the watch progress of the user as a fraction of the total watch time
 * @param streamID The ID of the currently watched stream
 * @param lastProgress The last progress fetched from the database
 */
export const watchProgress = function (this: VideoJsPlayer, streamID: number, lastProgress: number) {
  this.ready(() => {
    let duration: number;
    let timer: number;
    let iOSReady = false;
    let intervalInSeconds = 10000;

    // Fetch the user's video progress from the database and set the time in the player
    this.on('loadedmetadata', () => {
      duration = this.duration();
      this.currentTime(lastProgress * duration);
    });

    // iPhone/iPad need to set the progress again when they actually play the video. That's why loadedmetadata is
    // not sufficient here.
    // See https://stackoverflow.com/questions/28823567/how-to-set-currenttime-in-video-js-in-safari-for-ios.
    if (videojs.browser.IS_IOS) {
      this.on('canplaythrough', () => {
        // Can be executed multiple times during playback
        if (!iOSReady) {
          this.currentTime(lastProgress * duration);
          iOSReady = true;
        }
      });
    }

    const reportProgress = () => {
      const progress = this.currentTime() / duration;
      if (progress > 0) {
        fetch(`/api/streams/${streamID}/progress`, {
          method: 'POST',
          headers: {
            'Content-Type': 'application/json'
          },
          body: JSON.stringify({
            "streamID": streamID,
            "progress": progress
          })
        }).then(r => {
          if (r.status !== 200) {
            console.log(r);
            intervalInSeconds *= 2; // Binary exponential backoff for load balancing
          }
        });
      }
    }

    // Triggered when user presses play
    this.on('play', () => {
      const timer = setInterval(() => {
        reportProgress()
      }, intervalInSeconds);
    });

    // Triggered on pause and skipping the video
    this.on('pause', () => {
      clearInterval(timer);
      // "Bug" on iOS: The video is automatically paused at the beginning
      if (!iOSReady && videojs.browser.IS_IOS) {
        return;
      }
      reportProgress();
    });

    // Triggered when the video has no time left
    this.on('ended', () => {
      clearInterval(timer);
    });
  });
};
