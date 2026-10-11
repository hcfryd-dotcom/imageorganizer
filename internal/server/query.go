package server

import (
	"github.com/lkarlslund/imageorganizer/internal/store"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"
)

// yearRe matches a year (1800-2199) or a range of years written as
// 2000-2010, 2000..2010 or 2000–2010, as a whole word.
var yearRe = regexp.MustCompile(`(?:^|\s)((?:18|19|20|21)\d\d)(?:\s*(?:-|–|—|\.\.)\s*((?:18|19|20|21)\d\d))?(?:\s|$)`)

// DateRange is a half-open interval [From, To) in UTC.
type DateRange struct {
	From, To time.Time
	Label    string
}

// extractDates pulls years and year ranges out of a search query, returning
// the remaining text and the combined range (nil if none). Multiple years
// widen the range to cover all of them ("2010 2012" = 2010-2012).
func extractDates(q string) (string, *DateRange) {
	var lo, hi int
	for {
		m := yearRe.FindStringSubmatchIndex(q)
		if m == nil {
			break
		}
		a, _ := strconv.Atoi(q[m[2]:m[3]])
		b := a
		if m[4] >= 0 {
			b, _ = strconv.Atoi(q[m[4]:m[5]])
		}
		a, b = min(a, b), max(a, b)
		if lo == 0 || a < lo {
			lo = a
		}
		hi = max(hi, b)
		q = q[:m[0]] + " " + q[m[1]:]
	}
	q = strings.Join(strings.Fields(q), " ")
	if lo == 0 {
		return q, nil
	}
	label := strconv.Itoa(lo)
	if hi != lo {
		label += "–" + strconv.Itoa(hi)
	}
	return q, &DateRange{
		From:  time.Date(lo, 1, 1, 0, 0, 0, 0, time.UTC),
		To:    time.Date(hi+1, 1, 1, 0, 0, 0, 0, time.UTC),
		Label: label,
	}
}

// PersonName is a named person that search can recognise in a query.
type PersonName struct {
	ID   int64
	Name string
}

// connectors are words left dangling once names are taken out of a query
// ("Ben and Clara in the garden" → "in the garden").
var connectors = map[string]bool{"and": true, "og": true, "&": true, "+": true, ",": true, "with": true, "med": true, "of": true, "af": true}

// extractPeople pulls the names of known people out of a search query, as
// whole words and ignoring case, longest names first so "Anna Maria" wins
// over "Anna". It returns the remaining text and the matched people.
func extractPeople(q string, people []PersonName) (string, []PersonName) {
	people = slices.Clone(people)
	slices.SortFunc(people, func(a, b PersonName) int { return len(b.Name) - len(a.Name) })
	var found []PersonName
	pos := map[int64]int{} // where each person first appears, to keep query order
	for _, p := range people {
		name := strings.TrimSpace(p.Name)
		if name == "" {
			continue
		}
		for {
			i := indexWord(q, name)
			if i < 0 {
				break
			}
			if _, ok := pos[p.ID]; !ok {
				pos[p.ID] = i
				found = append(found, p)
			}
			q = q[:i] + strings.Repeat(" ", len(name)) + q[i+len(name):] // keep positions stable
		}
	}
	if len(found) == 0 {
		return q, nil
	}
	slices.SortFunc(found, func(a, b PersonName) int { return pos[a.ID] - pos[b.ID] })
	words := strings.Fields(strings.ReplaceAll(q, ",", " , "))
	for len(words) > 0 && connectors[strings.ToLower(words[0])] {
		words = words[1:]
	}
	for len(words) > 0 && connectors[strings.ToLower(words[len(words)-1])] {
		words = words[:len(words)-1]
	}
	return strings.ReplaceAll(strings.Join(words, " "), " ,", ","), found
}

// indexWord finds name in q as a whole word, ignoring case; -1 if absent.
func indexWord(q, name string) int {
	lq, ln := strings.ToLower(q), strings.ToLower(name)
	if len(lq) != len(q) || len(ln) != len(name) {
		return -1 // case folding changed byte lengths; positions would be off
	}
	for off := 0; ; {
		i := strings.Index(lq[off:], ln)
		if i < 0 {
			return -1
		}
		i += off
		before, _ := utf8.DecodeLastRuneInString(q[:i])
		after, _ := utf8.DecodeRuneInString(q[i+len(name):])
		if !isWordRune(before) && !isWordRune(after) {
			return i
		}
		off = i + 1
	}
}

func isWordRune(r rune) bool {
	return r != utf8.RuneError && (unicode.IsLetter(r) || unicode.IsDigit(r))
}

// specialRe matches the search filters: $known (a named person is in the
// photo, confirmed or not), $confirmed (one the user confirmed), $unconfirmed
// (one filed by itself), $unknown (someone not named yet), $alone (exactly
// one person is), $picture (a picture of someone is, on a wall), $nsfw and
// $blurry. one($kind) and many($kind) ask for exactly one, or two or more,
// of a face filter: one($known) one($unknown) is a named person with a
// stranger.
var specialRe = regexp.MustCompile(`(?i)(?:^|\s)(?:\$(known|unknown|confirmed|unconfirmed|alone|nsfw|blurry|picture)|(one|many)\(\s*\$?(known|unknown|confirmed|unconfirmed)\s*\))(?:\s|$)`)

// extractSpecial pulls the search filters out of a query, returning the
// remaining text and the filters found ("known", "one-unknown", ...).
func extractSpecial(q string) (string, []string) {
	var found []string
	for {
		m := specialRe.FindStringSubmatchIndex(q)
		if m == nil {
			break
		}
		w := ""
		if m[2] >= 0 {
			w = strings.ToLower(q[m[2]:m[3]])
		} else {
			w = strings.ToLower(q[m[4]:m[5]] + "-" + q[m[6]:m[7]])
		}
		if !slices.Contains(found, w) {
			found = append(found, w)
		}
		q = q[:m[0]] + " " + q[m[1]:]
	}
	return strings.Join(strings.Fields(q), " "), found
}

// faceFilter splits a face filter's name into its kind and count
// ("one-known" → "known", "one"); ok is false for other filters.
func faceFilter(v string) (kind, count string, ok bool) {
	for _, k := range store.FaceKinds {
		for _, c := range store.FaceCounts {
			if store.FaceFilterName(k, c) == v {
				return k, c, true
			}
		}
	}
	return "", "", false
}

// isSpecial reports whether v names a search filter.
func isSpecial(v string) bool {
	_, _, face := faceFilter(v)
	return face || v == "alone" || v == "nsfw" || v == "blurry" || v == "picture"
}
