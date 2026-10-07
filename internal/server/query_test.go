package server

import (
	"slices"
	"testing"
)

func TestExtractDates(t *testing.T) {
	for _, tc := range []struct{ in, text, label string }{
		{"2026", "", "2026"},
		{"beach 2026", "beach", "2026"},
		{"2000-2010", "", "2000–2010"},
		{"kids 2010..2005 snow", "kids snow", "2005–2010"},
		{"2015 – 2017 beach", "beach", "2015–2017"},
		{"2010 2012", "", "2010–2012"},
		{"IMG_2019.jpg", "IMG_2019.jpg", ""},
		{"room 101", "room 101", ""},
		{"20260", "20260", ""},
		{"", "", ""},
	} {
		text, r := extractDates(tc.in)
		label := ""
		if r != nil {
			label = r.Label
		}
		if text != tc.text || label != tc.label {
			t.Errorf("extractDates(%q) = %q, %q; want %q, %q", tc.in, text, label, tc.text, tc.label)
		}
	}
}

func TestExtractPeople(t *testing.T) {
	people := []PersonName{{1, "Ben"}, {2, "Clara"}, {3, "Anna"}, {4, "Anna Maria"}, {5, "Søren"}}
	for _, tc := range []struct {
		in, text string
		ids      []int64
	}{
		{"Ben in the garden", "in the garden", []int64{1}},
		{"ben and Clara on the beach", "on the beach", []int64{1, 2}},
		{"beach with Ben", "beach", []int64{1}},
		{"Anna Maria skiing", "skiing", []int64{4}},
		{"Anna, Ben", "", []int64{3, 1}},
		{"Bennett family", "Bennett family", nil},
		{"Søren", "", []int64{5}},
		{"garden", "garden", nil},
	} {
		text, found := extractPeople(tc.in, people)
		var ids []int64
		for _, p := range found {
			ids = append(ids, p.ID)
		}
		if text != tc.text || !slices.Equal(ids, tc.ids) {
			t.Errorf("extractPeople(%q) = %q, %v; want %q, %v", tc.in, text, ids, tc.text, tc.ids)
		}
	}
}

func TestExtractSpecial(t *testing.T) {
	for _, tc := range []struct {
		in, text string
		found    []string
	}{
		{"$unknown", "", []string{"unknown"}},
		{"beach $Known $unknown", "beach", []string{"known", "unknown"}},
		{"$unknown $unknown", "", []string{"unknown"}},
		{"price $5", "price $5", nil},
		{"a$unknown", "a$unknown", nil},
		{"Ben $alone beach", "Ben beach", []string{"alone"}},
	} {
		text, found := extractSpecial(tc.in)
		if text != tc.text || !slices.Equal(found, tc.found) {
			t.Errorf("extractSpecial(%q) = %q, %v; want %q, %v", tc.in, text, found, tc.text, tc.found)
		}
	}
}
