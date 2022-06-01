package worker

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/joschahenningsen/TUM-Live/worker/cfg"
	"github.com/joschahenningsen/TUM-Live/worker/pb"
	log "github.com/sirupsen/logrus"
)

type safeStreams struct {
	mutex   sync.Mutex
	streams map[uint32][]*StreamContext // Note that we can have multiple contexts for a streamID for different sources.
}

// regularStreams keeps track of all lecture hall streams for the current worker
var regularStreams = safeStreams{streams: make(map[uint32][]*StreamContext)}

// addContext adds a stream context for a given streamID to the map in safeStreams
func (s *safeStreams) addContext(streamID uint32, streamCtx *StreamContext) {
	s.mutex.Lock()
	defer s.mutex.Unlock()
	s.streams[streamID] = append(s.streams[streamCtx.streamId], streamCtx)
}

func HandlePremiere(request *pb.PremiereRequest) {
	streamCtx := &StreamContext{
		streamId:      request.StreamID,
		sourceUrl:     request.FilePath,
		startTime:     time.Now(),
		streamVersion: "",
		courseSlug:    "PREMIERE",
		stream:        true,
		commands:      nil,
		ingestServer:  request.IngestServer,
		outUrl:        request.OutUrl,
	}
	// Register worker for premiere
	if !streamCtx.isSelfStream {
		regularStreams.addContext(streamCtx.streamId, streamCtx)
	}
	S.startStream(streamCtx)
	streamPremiere(streamCtx)
	S.endStream(streamCtx)
	NotifyStreamDone(streamCtx)
}

func HandleSelfStream(request *pb.SelfStreamResponse, slug string) *StreamContext {
	streamCtx := &StreamContext{
		streamId:      request.GetStreamID(),
		courseSlug:    request.GetCourseSlug(),
		teachingTerm:  request.GetCourseTerm(),
		teachingYear:  request.GetCourseYear(),
		startTime:     request.GetStreamStart().AsTime().Local(),
		endTime:       time.Now().Add(time.Hour * 7),
		publishVoD:    request.GetUploadVoD(),
		stream:        true,
		streamVersion: "COMB",
		isSelfStream:  false,
		ingestServer:  request.IngestServer,
		sourceUrl:     "rtmp://localhost/stream/" + slug,
		streamName:    request.StreamName,
		outUrl:        request.OutUrl,
	}
	go stream(streamCtx)
	return streamCtx
}

func HandleSelfStreamRecordEnd(ctx *StreamContext) {
	S.startTranscoding(ctx.getStreamName())
	err := transcode(ctx)
	if err != nil {
		ctx.TranscodingSuccessful = false
		log.Errorf("Error while transcoding: %v", err)
	} else {
		ctx.TranscodingSuccessful = true
	}
	S.endTranscoding(ctx.getStreamName())
	notifyTranscodingDone(ctx)
	if ctx.publishVoD {
		upload(ctx)
		notifyUploadDone(ctx)
	}
	S.startSilenceDetection(ctx)
	defer S.endSilenceDetection(ctx)

	sd := NewSilenceDetector(ctx.getTranscodingFileName())
	err = sd.ParseSilence()
	if err != nil {
		log.WithField("File", ctx.getTranscodingFileName()).WithError(err).Error("Detecting silence failed.")
		return
	}
	notifySilenceResults(sd.Silences, ctx.streamId)

	err = CreateThumbnailSprite(ctx)
	if err != nil {
		log.WithField("File", ctx.getThumbnailFileName()).WithError(err).Error("Creating thumbnail sprite failed.")
		return
	}
	if ctx.TranscodingSuccessful {
		err := markForDeletion(ctx)
		if err != nil {
			log.WithField("stream", ctx.streamId).WithError(err).Error("Error marking for deletion")
		}
	}
}

// HandleStreamEndRequest ends all streams for a given streamID contained in request
func HandleStreamEndRequest(request *pb.EndStreamRequest) {
	log.Info("Attempting to end stream: ", request.StreamID)
	regularStreams.endStreams(request)
}

func (s *safeStreams) endStreams(request *pb.EndStreamRequest) {
	s.mutex.Lock()
	defer s.mutex.Unlock()
	stream := s.streams[request.StreamID]
	for _, streamContext := range stream {
		streamContext.discardVoD = request.DiscardVoD
		HandleStreamEnd(streamContext)
	}
	// All streams should be ended right now, so we can delete them
	delete(s.streams, request.StreamID)
}

// HandleStreamEnd stops the ffmpeg instance by sending a SIGINT to it and prevents the loop to restart it by marking the stream context as stopped.
func HandleStreamEnd(ctx *StreamContext) {
	ctx.stopped = true
	if ctx.streamCmd != nil && ctx.streamCmd.Process != nil {
		pgid, err := syscall.Getpgid(ctx.streamCmd.Process.Pid)
		if err != nil {
			log.WithError(err).WithField("streamID", ctx.streamId).Warn("Can't find pgid for ffmpeg")
		} else {
			// We use the new pgid that we created in stream.go to actually interrupt the shell process with all its children
			err := syscall.Kill(-pgid, syscall.SIGINT) // Note that the - is used to kill process groups
			if err != nil {
				log.WithError(err).WithField("streamID", ctx.streamId).Warn("Can't interrupt ffmpeg")
			}
		}
	} else {
		log.Warn("context has no command or process to end")
	}
}

func HandleStreamRequest(request *pb.StreamRequest) {
	log.WithField("request", request).Info("Request to stream")
	//setup context with relevant information to pass to other subprocesses
	streamCtx := &StreamContext{
		streamId:      request.GetStreamID(),
		sourceUrl:     "rtsp://" + request.GetSourceUrl(),
		courseSlug:    request.GetCourseSlug(),
		teachingTerm:  request.GetCourseTerm(),
		teachingYear:  request.GetCourseYear(),
		startTime:     request.GetStart().AsTime().Local(),
		endTime:       request.GetEnd().AsTime().Local(),
		streamVersion: request.GetSourceType(),
		publishVoD:    request.GetPublishVoD(),
		stream:        request.GetPublishStream(),
		streamName:    request.GetStreamName(),
		ingestServer:  request.GetIngestServer(),
		isSelfStream:  false,
		outUrl:        request.GetOutUrl(),
	}

	// Register worker for stream
	regularStreams.addContext(streamCtx.streamId, streamCtx)

	//only record
	if !streamCtx.stream {
		S.startRecording(streamCtx.getRecordingFileName())
		record(streamCtx)
		S.endRecording(streamCtx.getRecordingFileName())
	} else {
		stream(streamCtx)
	}
	NotifyStreamDone(streamCtx) // notify stream/recording done
	if streamCtx.discardVoD {
		log.Info("Skipping VoD creation")
		return
	}
	S.startTranscoding(streamCtx.getStreamName())
	err := transcode(streamCtx)
	if err != nil {
		streamCtx.TranscodingSuccessful = false
		log.Errorf("Error while transcoding: %v", err)
	} else {
		streamCtx.TranscodingSuccessful = true
	}
	S.endTranscoding(streamCtx.getStreamName())
	notifyTranscodingDone(streamCtx)

	err = CreateThumbnailSprite(streamCtx)
	if err != nil {
		log.WithField("File", streamCtx.getTranscodingFileName()).WithError(err).Error("Creating thumbnail sprite failed")
	}

	if request.PublishVoD {
		upload(streamCtx)
		notifyUploadDone(streamCtx)
	}

	err = CreateThumbnailSprite(streamCtx)
	if err != nil {
		log.WithField("File", streamCtx.getThumbnailFileName()).WithError(err).Error("Creating thumbnail sprite failed")
	}
	notifyThumbnailDone(streamCtx)

	if streamCtx.streamVersion == "COMB" {
		S.startSilenceDetection(streamCtx)
		defer S.endSilenceDetection(streamCtx)
		sd := NewSilenceDetector(streamCtx.getTranscodingFileName())
		err := sd.ParseSilence()
		if err != nil {
			log.WithField("File", streamCtx.getTranscodingFileName()).WithError(err).Error("Detecting silence failed.")
			return
		}
		notifySilenceResults(sd.Silences, streamCtx.streamId)
	}
	if streamCtx.TranscodingSuccessful {
		err := markForDeletion(streamCtx)
		if err != nil {
			log.WithField("stream", streamCtx.streamId).WithError(err).Error("Error marking for deletion")
		}
	}
}

func HandleUploadRestReq(uploadKey string, localFile string) {
	client, conn, err := GetClient()
	if err != nil {
		log.WithError(err).Error("Error getting client")
		return
	}
	defer closeConnection(conn)
	resp, err := client.GetStreamInfoForUpload(context.Background(), &pb.GetStreamInfoForUploadRequest{
		UploadKey: uploadKey,
		WorkerID:  cfg.WorkerID,
	})
	if err != nil {
		log.WithError(err).Error("Error getting stream info for upload")
		return
	}
	c := StreamContext{
		streamId:      resp.StreamID,
		courseSlug:    resp.CourseSlug,
		teachingTerm:  resp.CourseTerm,
		teachingYear:  resp.CourseYear,
		startTime:     resp.StreamStart.AsTime(),
		endTime:       resp.StreamEnd.AsTime(),
		streamVersion: "COMB",
		publishVoD:    true,
		recordingPath: &localFile,
	}
	log.WithFields(log.Fields{"stream": c.streamId, "course": c.courseSlug, "file": localFile}).Debug("Handling upload request")

	needsConversion := false

	if container, err := getContainer(localFile); err != nil {
		log.WithError(err).Error("Error getting container")
		needsConversion = true
	} else if !strings.Contains(container, "mp4") {
		needsConversion = true
		log.Debugf("Wrong container: %s, converting", container)
	}

	if codec, err := getCodec(localFile); err != nil {
		log.WithError(err).Warn("Error getting codec")
		needsConversion = true
	} else if codec != "h264" {
		needsConversion = true
		log.Debugf("wrong codec: %s, converting", codec)
	}

	level, err := getLevel(localFile)
	if err != nil {
		log.WithError(err).Warn("Error getting level")
		needsConversion = true
	}
	if levelInt, err := strconv.Atoi(level); err != nil {
		log.WithError(err).Warnf("Error converting level(%s) to int", level)
		needsConversion = true
	} else {
		if levelInt > 42 {
			needsConversion = true
			log.Debugf("Level too high: %d, converting", levelInt)
		}
	}

	if needsConversion {
		log.WithField("stream", c.streamId).Debug("Converting video from upload request")
		S.startTranscoding(c.getStreamName())
		err := transcode(&c)
		if err != nil {
			log.WithError(err).Error("Error transcoding")
		}
		notifyTranscodingDone(&c)
		S.endTranscoding(c.getStreamName())

	} else {
		log.WithField("stream", c.streamId).Debug("Not converting video from upload request")
		// create required directories
		log.WithField("transcodingFileName", c.getTranscodingFileName()).Debug("Creating output directory")
		if err = prepare(c.getTranscodingFileName()); err != nil {
			log.Error(err)
		}
		log.WithFields(log.Fields{"in": c.getRecordingTrashName(), "out": c.getTranscodingFileName()}).Debug("Copying file")
		if err = moveFile(c.getRecordingFileName(), c.getTranscodingFileName()); err != nil {
			log.WithError(err).Error("Can't move upload to transcoding dir")
		} else {
			log.WithField("stream", c.streamId).Debug("Successfully moved upload to target dir")
		}
	}
	err = CreateThumbnailSprite(&c)
	if err != nil {
		log.WithField("File", c.getThumbnailFileName()).WithError(err).Error("Creating thumbnail sprite failed")
	}
	notifyThumbnailDone(&c)

	S.startSilenceDetection(&c)
	defer S.endSilenceDetection(&c)
	sd := NewSilenceDetector(c.getTranscodingFileName())
	if err = sd.ParseSilence(); err != nil {
		log.WithField("File", c.getTranscodingFileName()).WithError(err).Error("Detecting silence failed.")
	} else {
		notifySilenceResults(sd.Silences, c.streamId)
	}

	upload(&c)
	notifyUploadDone(&c)
	_ = os.Remove(c.getRecordingFileName())
}

// moveFile moves a file from sourcePath to destPath.
// in contrast to os.Rename, this function can copy files from one drive to another.
func moveFile(sourcePath, destPath string) error {
	inputFile, err := os.Open(sourcePath)
	if err != nil {
		return fmt.Errorf("open source file: %s", err)
	}
	outputFile, err := os.Create(destPath)
	if err != nil {
		inputFile.Close()
		return fmt.Errorf("open dest file: %s", err)
	}
	defer outputFile.Close()
	_, err = io.Copy(outputFile, inputFile)
	inputFile.Close()
	if err != nil {
		return fmt.Errorf("writing to output file: %s", err)
	}
	// The copy was successful, so now delete the original file
	err = os.Remove(sourcePath)
	if err != nil {
		return fmt.Errorf("removing original file: %s", err)
	}
	return nil
}

// StreamContext contains all important information on a stream
type StreamContext struct {
	streamId      uint32         //id of the stream
	sourceUrl     string         //url of the streams source, e.g. 10.0.0.4
	courseSlug    string         //slug of the course, e.g. eidi
	teachingTerm  string         //S or W depending on the courses teaching-term
	teachingYear  uint32         //Year the course takes place in
	startTime     time.Time      //time the stream should start
	endTime       time.Time      //end of the stream (including +10 minute safety)
	streamVersion string         //version of the stream to be handled, e.g. PRES, COMB or CAM
	publishVoD    bool           //whether file should be uploaded
	stream        bool           //whether streaming is enabled
	commands      map[string]int //map command type to pid, e.g. "stream"->123
	streamCmd     *exec.Cmd      // command used for streaming
	isSelfStream  bool           //deprecated
	streamName    string         // ingest target
	ingestServer  string         // ingest tumlive e.g. rtmp://user:password@my.tumlive
	stopped       bool           // whether the stream has been stopped
	outUrl        string         // url the stream will be available at
	discardVoD    bool           // whether the VoD should be discarded

	// calculated after stream:
	duration uint32 //duration of the stream in seconds

	TranscodingSuccessful bool // TranscodingSuccessful is true if the transcoding was successful

	thumbnailSpritePath string  // path to the thumbnail sprite
	recordingPath       *string // recordingPath: path to the recording (overrides default path if set)
}

// getRecordingFileName returns the filename a stream should be saved to before transcoding.
// example: /recordings/eidi_2021-09-23_10-00_COMB.ts
func (s StreamContext) getRecordingFileName() string {
	if s.recordingPath != nil {
		return *s.recordingPath
	}
	if !s.isSelfStream {
		return fmt.Sprintf("%s/%s.ts",
			cfg.TempDir,
			s.getStreamName())
	}
	return fmt.Sprintf("%s/%s_%s.flv",
		cfg.TempDir,
		s.courseSlug,
		s.startTime.Format("02012006"))
}

func (s StreamContext) getRecordingTrashName() string {
	fn := s.getRecordingFileName()
	return filepath.Join(filepath.Dir(fn), ".trash", filepath.Base(fn))
}

// getTranscodingFileName returns the filename a stream should be saved to after transcoding.
// example: /srv/sharedMassStorage/2021/S/eidi/2021-09-23_10-00/eidi_2021-09-23_10-00_PRES.mp4
func (s StreamContext) getTranscodingFileName() string {
	if s.isSelfStream {
		return fmt.Sprintf("%s/%d/%s/%s/%s/%s-%s.mp4",
			cfg.StorageDir,
			s.teachingYear,
			s.teachingTerm,
			s.courseSlug,
			s.startTime.Format("2006-01-02_15-04"),
			s.courseSlug,
			s.startTime.Format("02012006"))
	}
	return fmt.Sprintf("%s/%d/%s/%s/%s/%s.mp4",
		cfg.StorageDir,
		s.teachingYear,
		s.teachingTerm,
		s.courseSlug,
		s.startTime.Format("2006-01-02_15-04"),
		s.getStreamName())
}

// getTranscodingFileName returns the filename a stream should be saved to after transcoding.
// example: /srv/sharedMassStorage/2021/S/eidi/2021-09-23_10-00/eidi_2021-09-23_10-00_PRES.mp4
func (s StreamContext) getThumbnailFileName() string {
	if s.isSelfStream {
		return fmt.Sprintf("%s/%d/%s/%s/%s/%s-%s-thumb.jpg",
			cfg.StorageDir,
			s.teachingYear,
			s.teachingTerm,
			s.courseSlug,
			s.startTime.Format("2006-01-02_15-04"),
			s.courseSlug,
			s.startTime.Format("02012006"))
	}
	return fmt.Sprintf("%s/%d/%s/%s/%s/%s-thumb.jpg",
		cfg.StorageDir,
		s.teachingYear,
		s.teachingTerm,
		s.courseSlug,
		s.startTime.Format("2006-01-02_15-04"),
		s.getStreamName())
}

// getStreamName returns the stream name, used for the worker status
func (s StreamContext) getStreamName() string {
	if !s.isSelfStream {
		return fmt.Sprintf("%s-%s%s",
			s.courseSlug,
			s.startTime.Format("2006-01-02-15-04"),
			s.streamVersion)
	}
	return s.courseSlug
}

// getStreamNameVoD returns the stream name for vod (lrz replaces - with _)
func (s StreamContext) getStreamNameVoD() string {
	if !s.isSelfStream {
		return strings.ReplaceAll(fmt.Sprintf("%s_%s%s",
			s.courseSlug,
			s.startTime.Format("2006_01_02_15_04"),
			s.streamVersion), "-", "_")
	}
	return s.courseSlug
}
