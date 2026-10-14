package media

import (
	"encoding/json"
	"testing"
)

func TestFromProbe(t *testing.T) {
	var p probe
	// An iPhone video filmed upright: stored landscape, turned 90° on play.
	json.Unmarshal([]byte(`{
		"format": {"duration": "24.08", "tags": {
			"creation_time": "2015-07-27T19:13:44.000000Z",
			"com.apple.quicktime.creationdate": "2015-07-27T22:13:44+0300",
			"com.apple.quicktime.location.ISO6709": "+36.7941+031.3738+017.468/",
			"com.apple.quicktime.make": "Apple", "com.apple.quicktime.model": "iPhone 5s"}},
		"streams": [{"codec_type": "audio"},
			{"codec_type": "video", "width": 1920, "height": 1080, "side_data_list": [{"rotation": -90}]}]}`), &p)
	var in Info
	in.fromProbe(p)
	if in.Width != 1080 || in.Height != 1920 {
		t.Errorf("size %dx%d, want upright 1080x1920", in.Width, in.Height)
	}
	if in.TakenAt == nil || in.TakenAt.Format("2006-01-02 15:04:05") != "2015-07-27 22:13:44" {
		t.Errorf("taken %v, want the phone's local 22:13:44", in.TakenAt)
	}
	if in.Lat == nil || *in.Lat != 36.7941 || *in.Lon != 31.3738 {
		t.Errorf("position %v %v", in.Lat, in.Lon)
	}
	if in.CameraMake != "Apple" || in.CameraModel != "iPhone 5s" || in.Duration < 24 {
		t.Errorf("%q %q %.1f", in.CameraMake, in.CameraModel, in.Duration)
	}
	// Android: an ISO 6709 "location" and only a UTC time.
	p = probe{}
	json.Unmarshal([]byte(`{"format": {"tags": {"creation_time": "2020-01-02T03:04:05Z", "location": "-33.8688+151.2093/"}},
		"streams": [{"codec_type": "video", "width": 640, "height": 480}]}`), &p)
	in = Info{}
	in.fromProbe(p)
	if in.TakenAt == nil || in.Lat == nil || *in.Lat != -33.8688 || in.Width != 640 {
		t.Errorf("android: %+v", in)
	}
}

func TestVideoNeedsEnabling(t *testing.T) {
	if IsVideo("a.mov") && ffmpeg == "" {
		t.Error("videos indexed without ffmpeg")
	}
	if _, err := Process("a.mov", nil, false, 0); err == nil {
		t.Error("Process took a video")
	}
}
