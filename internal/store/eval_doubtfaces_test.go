package store

import (
	"fmt"
	"os"
	"testing"
	"time"
)

// TestEvalDoubtfulFaces counts the faces DoubtfulFaces would ask about on
// a database copy, by reason, and how many of the user's own tagged faces
// (right, all of them) the same tests would doubt: IO_EVAL_DB=<copy>.
func TestEvalDoubtfulFaces(t *testing.T) {
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
	tagged, filed := 0, 0
	for _, f := range s.faces {
		if c := s.clusters[f.ClusterID]; c != nil && c.PersonID != 0 && !f.NotFace && !f.Picture && s.facePx(f) >= fitMinPx {
			if f.Tagged == c.PersonID {
				tagged++
			} else if f.Tagged == 0 {
				filed++
			}
		}
	}
	for _, ctl := range []bool{false, true} {
		start := time.Now()
		which := doubtFiled
		if ctl {
			which = doubtTagged
		}
		ds := s.doubtfulFaces(0, doubtOpts{which: which}, true)
		var age, gender, alike, other, auto int
		for _, d := range ds {
			if d.AgeOff {
				age++
			}
			if d.Gender != "" {
				gender++
			}
			if d.Fit < fitAlike {
				alike++
			}
			if d.Other != 0 {
				other++
			}
			if d.Auto {
				auto++
			}
		}
		of := filed
		what := "filed by group"
		if ctl {
			of, what = tagged, "TAGGED (all right)"
		}
		t.Logf("%-20s %5d doubted of %6d (%.2f%%): age %d, gender %d, unlike %d, more like another %d, auto %d — %v",
			what, len(ds), of, 100*float32(len(ds))/float32(max(of, 1)), age, gender, alike, other, auto, time.Since(start))
		if !ctl {
			for i, d := range ds {
				if i < 30 {
					fmt.Printf("%3d %-18s fit %.2f looks %2.0f born %4.0f photo %4.0f ageoff %-5v gender %-6s other %-14s %.2f auto %v\n",
						i+1, d.Name, d.Fit, d.Looks, d.Born, d.Year, d.AgeOff, d.Gender, d.Name2, d.OtherFit, d.Auto)
				}
			}
		}
	}
}
