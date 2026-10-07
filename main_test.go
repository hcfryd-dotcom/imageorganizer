package main

import (
	"testing"
	"time"

	"github.com/lkarlslund/imageorganizer/internal/store"
)

func TestSettingsFillInFlags(t *testing.T) {
	flags := config{embedURL: "http://a:1", workers: 4, ortLib: "/flag/lib.so", rescan: 30 * time.Minute}
	s := store.Settings{EmbedURL: "http://b:2", EmbedModel: "m", Workers: 8, ONNXRuntime: "/saved/lib.so", RescanMinutes: -1}
	c := flags.withSettings(s, map[string]bool{"onnxruntime": true})
	if c.embedURL != "http://b:2" || c.embedModel != "m" || c.workers != 8 {
		t.Errorf("saved settings should fill in flags not given: %+v", c)
	}
	if c.ortLib != "/flag/lib.so" {
		t.Errorf("a flag given on the command line wins: %q", c.ortLib)
	}
	if c.rescan != 0 {
		t.Errorf("rescan -1 means never: %v", c.rescan)
	}
	if off := flags.withSettings(store.Settings{EmbedURL: "off"}, nil); off.embedURL != "" {
		t.Errorf("embed URL off disables search by meaning: %q", off.embedURL)
	}
	if !c.needsRestart(flags) || flags.needsRestart(flags) {
		t.Error("needsRestart should compare start-only settings")
	}
}
