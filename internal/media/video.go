package media

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/cespare/xxhash/v2"
)

// Videos are taken in through ffmpeg: ffprobe for when and where one was
// filmed and with what, ffmpeg for a frame near the start, which then goes
// through Process like a photo (thumbnail, perceptual hash, faces). Video
// files are only indexed once EnableVideo has found the two programs.

// videoExts are the video files indexed when video is enabled.
var videoExts = []string{".mov", ".mp4", ".m4v", ".3gp", ".avi", ".mkv", ".mts", ".m2ts", ".wmv", ".webm", ".mpg", ".mpeg"}

var ffmpeg, ffprobe string

// EnableVideo indexes video files from now on, using these programs.
func EnableVideo(ffmpegPath, ffprobePath string) {
	ffmpeg, ffprobe = ffmpegPath, ffprobePath
	for _, e := range videoExts {
		extensions[e] = kindVideo
	}
}

// IsVideo reports whether path is a video file this program indexes.
func IsVideo(path string) bool {
	k, ok := extensions[strings.ToLower(filepath.Ext(path))]
	return ok && k == kindVideo
}

// VideoFormat reports whether a format (file extension without the dot)
// is a video's.
func VideoFormat(format string) bool { return IsVideo("x." + format) }

// HashReader is the content hash the library knows files by (xxhash and
// length), read in pieces so a video needn't fit in memory.
func HashReader(r io.Reader) (string, int64, error) {
	h := xxhash.New()
	n, err := io.Copy(h, r)
	if err != nil {
		return "", n, err
	}
	return fmt.Sprintf("%016x%08x", h.Sum64(), n), n, nil
}

// probe is the part of ffprobe's JSON this reads.
type probe struct {
	Format struct {
		Duration string            `json:"duration"`
		Tags     map[string]string `json:"tags"`
	} `json:"format"`
	Streams []struct {
		CodecType string            `json:"codec_type"`
		Width     int               `json:"width"`
		Height    int               `json:"height"`
		Tags      map[string]string `json:"tags"`
		SideData  []struct {
			Rotation float64 `json:"rotation"`
		} `json:"side_data_list"`
	} `json:"streams"`
}

// VideoInfo is what a video file says of itself.
func VideoInfo(ctx context.Context, path string) (Info, error) {
	var in Info
	in.Format = strings.TrimPrefix(strings.ToLower(filepath.Ext(path)), ".")
	if ffprobe == "" {
		return in, errors.New("video is not enabled")
	}
	ctx, cancel := context.WithTimeout(ctx, time.Minute)
	defer cancel()
	out, err := exec.CommandContext(ctx, ffprobe, "-v", "quiet", "-print_format", "json", "-show_format", "-show_streams", path).Output()
	if err != nil {
		return in, fmt.Errorf("ffprobe: %w", err)
	}
	var p probe
	if err := json.Unmarshal(out, &p); err != nil {
		return in, fmt.Errorf("ffprobe: %w", err)
	}
	in.fromProbe(p)
	return in, nil
}

// iso6709 is a position like "+55.6761+012.5683+010.000/".
var iso6709 = regexp.MustCompile(`^([+-]\d+(?:\.\d+)?)([+-]\d+(?:\.\d+)?)`)

func (in *Info) fromProbe(p probe) {
	tags := map[string]string{}
	for k, v := range p.Format.Tags {
		tags[strings.ToLower(k)] = v
	}
	if d, err := strconv.ParseFloat(p.Format.Duration, 64); err == nil && d > 0 {
		in.Duration = float32(d)
	}
	for _, s := range p.Streams {
		if s.CodecType != "video" || s.Width == 0 {
			continue
		}
		in.Width, in.Height = s.Width, s.Height
		rot := 0.0
		if r, err := strconv.ParseFloat(s.Tags["rotate"], 64); err == nil {
			rot = r
		}
		for _, sd := range s.SideData {
			if sd.Rotation != 0 {
				rot = sd.Rotation
			}
		}
		if r := int(math.Abs(rot)) % 180; r == 90 { // filmed upright: shown turned
			in.Width, in.Height = in.Height, in.Width
		}
		if t := s.Tags["creation_time"]; t != "" && tags["creation_time"] == "" {
			tags["creation_time"] = t
		}
		break
	}
	// When: the phone's local time if it gives one (as the camera's clock
	// in a photo's EXIF), else the UTC time in this computer's time zone.
	// Times are kept as wall-clock, as EXIF dates are.
	wall := func(t time.Time) *time.Time {
		w := time.Date(t.Year(), t.Month(), t.Day(), t.Hour(), t.Minute(), t.Second(), 0, time.UTC)
		return &w
	}
	if v := tags["com.apple.quicktime.creationdate"]; v != "" {
		if t, err := time.Parse("2006-01-02T15:04:05-0700", v); err == nil {
			in.TakenAt = wall(t)
		}
	}
	if in.TakenAt == nil {
		if t, err := time.Parse(time.RFC3339Nano, tags["creation_time"]); err == nil && t.Year() > 1970 {
			in.TakenAt = wall(t.In(time.Local))
		}
	}
	for _, k := range []string{"com.apple.quicktime.location.iso6709", "location"} {
		if m := iso6709.FindStringSubmatch(tags[k]); m != nil {
			lat, err1 := strconv.ParseFloat(m[1], 64)
			lon, err2 := strconv.ParseFloat(m[2], 64)
			if err1 == nil && err2 == nil && (lat != 0 || lon != 0) && math.Abs(lat) <= 90 && math.Abs(lon) <= 180 {
				in.Lat, in.Lon = &lat, &lon
				break
			}
		}
	}
	for _, k := range []string{"com.apple.quicktime.make", "make", "com.android.manufacturer"} {
		if v := strings.TrimSpace(tags[k]); v != "" {
			in.CameraMake = v
			break
		}
	}
	for _, k := range []string{"com.apple.quicktime.model", "model", "com.android.model"} {
		if v := strings.TrimSpace(tags[k]); v != "" {
			in.CameraModel = v
			break
		}
	}
}

// videoFrame is a frame near the start of the video as a JPEG, upright,
// with a long edge of at most WorkSize.
func videoFrame(ctx context.Context, path string, dur float32) ([]byte, error) {
	at := 1.0
	if dur > 0 && dur < 2 {
		at = float64(dur) / 2
	}
	ctx, cancel := context.WithTimeout(ctx, 2*time.Minute)
	defer cancel()
	scale := fmt.Sprintf("scale='if(gt(iw,ih),min(%d,iw),-2)':'if(gt(iw,ih),-2,min(%d,ih))'", WorkSize, WorkSize)
	cmd := exec.CommandContext(ctx, ffmpeg, "-v", "error", "-ss", strconv.FormatFloat(at, 'f', 2, 64), "-i", path,
		"-frames:v", "1", "-vf", scale, "-f", "image2pipe", "-vcodec", "mjpeg", "-q:v", "3", "pipe:1")
	var out, stderr bytes.Buffer
	cmd.Stdout, cmd.Stderr = &out, &stderr
	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("ffmpeg: %v: %s", err, strings.TrimSpace(stderr.String()))
	}
	if out.Len() == 0 && at > 0 { // shorter than it said: the first frame
		return videoFrame(ctx, path, 0.01)
	}
	if out.Len() == 0 {
		return nil, errors.New("ffmpeg: no frame")
	}
	return out.Bytes(), nil
}

// ProcessVideo is Process for a video file on this computer: its own
// metadata, and the thumbnail (and analysis copy) of a frame near the
// start, turned rot degrees more.
func ProcessVideo(ctx context.Context, path string, wantWork bool, rot int) (Result, error) {
	in, err := VideoInfo(ctx, path)
	res := Result{Info: in}
	if err != nil {
		return res, err
	}
	frame, err := videoFrame(ctx, path, in.Duration)
	if err != nil {
		return res, err
	}
	r, err := Process("frame.jpg", frame, wantWork, rot)
	if err != nil {
		return res, err
	}
	res.Thumb, res.PHash, res.Work = r.Thumb, r.PHash, r.Work
	if res.Info.Width == 0 {
		res.Info.Width, res.Info.Height = r.Info.Width, r.Info.Height
	} else if rot == 90 || rot == 270 {
		res.Info.Width, res.Info.Height = res.Info.Height, res.Info.Width
	}
	return res, nil
}
