package store

import (
	"fmt"
	"os"
	"slices"
	"testing"
	"time"
)

// TestEvalScanOrder prints a scan session in scan order with the year its
// faces say, to see whether neighbours agree: IO_EVAL_DB, IO_EVAL_DAY.
func TestEvalScanOrder(t *testing.T) {
	path, day := os.Getenv("IO_EVAL_DB"), os.Getenv("IO_EVAL_DAY")
	if path == "" || day == "" {
		t.Skip("set IO_EVAL_DB and IO_EVAL_DAY")
	}
	s, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	s.mu.RLock()
	defer s.mu.RUnlock()
	says := map[int64]int{}
	for _, d := range s.doubtfulDates(0) {
		says[d.Image] = d.Year
	}
	var ids []int64
	for id, im := range s.images {
		if im.TakenAt != nil && time.Unix(*im.TakenAt, 0).UTC().Format("2006-01-02") == day {
			ids = append(ids, id)
		}
	}
	slices.SortFunc(ids, func(a, b int64) int { return int(*s.images[a].TakenAt - *s.images[b].TakenAt) })
	line := ""
	for i, id := range ids {
		y := "  .  "
		if v, ok := says[id]; ok {
			y = fmt.Sprintf(" %d", v)
		} else if len(s.facesByImage[id]) > 0 {
			y = "  f  "
		}
		line += y
		if (i+1)%16 == 0 {
			fmt.Println(line)
			line = ""
		}
	}
	fmt.Println(line)
	t.Logf("%d photos that day, %d dated by faces; first %s last %s", len(ids), len(says), time.Unix(*s.images[ids[0]].TakenAt, 0).UTC().Format("15:04:05"), time.Unix(*s.images[ids[len(ids)-1]].TakenAt, 0).UTC().Format("15:04:05"))
}
