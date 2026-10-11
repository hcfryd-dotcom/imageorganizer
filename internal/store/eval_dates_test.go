package store

import (
	"fmt"
	"os"
	"strings"
	"testing"
	"time"
)

// TestEvalDates lists the photos whose date the faces in them disagree
// with, on a database copy: IO_EVAL_DB=<copy>.
func TestEvalDates(t *testing.T) {
	path := os.Getenv("IO_EVAL_DB")
	if path == "" {
		t.Skip("set IO_EVAL_DB")
	}
	s, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	s.mu.RLock()
	defer s.mu.RUnlock()
	start := time.Now()
	ds := s.doubtfulDates(0)
	t.Logf("%d doubtful of %d photos with faces, in %v", len(ds), len(s.facesByImage), time.Since(start))
	byDay := map[string]int{}
	unborn, scans := 0, 0
	for i, d := range ds {
		if d.Unborn != "" {
			unborn++
		}
		if d.Scanned {
			scans++
		}
		day := time.Unix(d.Taken, 0).UTC().Format("2006-01-02")
		byDay[day]++
		if i < 40 && os.Getenv("IO_EVAL_ALL") == "" || !d.Scanned {
			var fs []string
			for _, f := range d.Faces {
				fs = append(fs, fmt.Sprintf("%s b%.0f looks %.0f", f.Name, f.Born, f.Looks))
			}
			fmt.Printf("%4d img %d dated %s → %d ±%.1f unborn %q scanned %v | %s\n", i+1, d.Image, day, d.Year, d.Err, d.Unborn, d.Scanned, strings.Join(fs, "; "))
		}
	}
	t.Logf("someone unborn: %d, scanner dates: %d, on %d days", unborn, scans, len(byDay))
	n := 0
	for day, c := range byDay {
		if c >= 10 {
			fmt.Printf("day %s: %d photos\n", day, c)
			n++
		}
	}
}
