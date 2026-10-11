package store

import (
	"cmp"
	"context"
	"math"
	"slices"
	"time"
)

// Photos whose date is wrong: an old print photographed or scanned years
// later carries the day it was copied. The people in it give it away: how
// old they look, and when they were born, says when it was taken.
const (
	dateOff    = 6    // years off, at least, to doubt a photo's date
	dateSure   = 3    // and that many times the uncertainty of the faces' year
	dateUnborn = 2    // years before someone's birth a photo can't be from
	dateMinPx  = 40   // faces smaller than this don't tell their age well enough
	dateKidAge = 1.5  // uncertainty of the year a face gives, in years: this
	dateAgeErr = 0.2  // plus this much per year of its age (adults are read loosely)
	dateMinN   = 8    // dated faces a person needs for their birth year to count
	dateAgree  = 2.5  // uncertainties a face's year may be from the others', for them to agree
	dateLike   = 0.70 // a face not named yet this like someone born after the photo dates it too
)

// DateDoubt is a photo whose date the faces in it disagree with.
type DateDoubt struct {
	Image   int64      `json:"image"`
	Taken   int64      `json:"taken"`   // the date it has now
	Year    int        `json:"year"`    // the year its faces say
	Err     float32    `json:"err"`     // give or take, in years
	Unborn  string     `json:"unborn"`  // someone in it not born then
	Scanned bool       `json:"scanned"` // its date is a scanner's
	Faces   []DateFace `json:"faces"`
	off     float32
}

// DateFace is a face that dates a photo: who, born when, looking how old.
type DateFace struct {
	FaceView
	Name  string  `json:"name"`
	Born  float32 `json:"born"`
	Looks float32 `json:"looks"`
}

// DoubtfulDates lists photos whose date the people in them disagree with,
// surest first, with the year they say. Photos whose date the user set or
// said is right aren't listed.
func (s *Store) DoubtfulDates(ctx context.Context, limit int) []DateDoubt {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.doubtfulDates(limit)
}

// bornFromFaces is each person's birth year: entered, or the median of
// what their dated faces say: the ones the user confirmed when there are
// enough, else all filed under them too (a few misdated photos don't move
// a median; a sibling's faces filed by themselves could). Call with s.mu
// held.
func (s *Store) bornFromFaces() map[int64]float32 {
	says, sure := map[int64][]float32{}, map[int64][]float32{} // all, and confirmed
	for _, f := range s.faces {
		im := s.images[f.ImageID]
		if im == nil || im.TakenAt == nil || scanned(im) || !s.shown(f) || f.Picture || f.Age < 0 || s.facePx(f) < dateMinPx {
			continue
		}
		p := f.Tagged
		if c := s.clusters[f.ClusterID]; p == 0 && c != nil {
			p = c.PersonID
		}
		if p != 0 {
			says[p] = append(says[p], yearOf(*im.TakenAt)-f.Age)
			if f.Tagged == p {
				sure[p] = append(sure[p], yearOf(*im.TakenAt)-f.Age)
			}
		}
	}
	// The confirmed faces alone, once there are enough: faces filed by
	// themselves mustn't move it (see ageModels).
	for p, ys := range sure {
		if len(ys) >= dateMinN {
			says[p] = ys
		}
	}
	born := map[int64]float32{}
	for id, p := range s.people {
		switch {
		case p.Picture:
		case p.BirthYear != 0:
			born[id] = float32(p.BirthYear) + 0.5
		case len(says[id]) >= dateMinN:
			ys := says[id]
			slices.Sort(ys)
			born[id] = ys[len(ys)/2]
		}
	}
	return born
}

func (s *Store) doubtfulDates(limit int) []DateDoubt {
	born := s.bornFromFaces()
	// Faces not named yet, but very like someone born after the photo's
	// date, say so too: they can't be named as them (their age then
	// says no) until the date is put right.
	refs := map[int64][][]float32{}
	var latest float32
	for pid, pi := range s.personIndex() {
		if b, ok := born[pid]; ok && !pi.p.Picture {
			for _, g := range pi.groups {
				refs[pid] = append(refs[pid], clusterUnit(g))
			}
			latest = max(latest, b)
		}
	}
	var out []DateDoubt
	for img := range s.facesByImage {
		im := s.images[img]
		if im == nil || im.TakenAt == nil || im.UserTaken != nil || im.DateOK {
			continue
		}
		year := yearOf(*im.TakenAt)
		d := DateDoubt{Image: img, Taken: *im.TakenAt, Scanned: scanned(im)}
		var sum, w float64
		var unborn float32
		for p, f := range s.whoIn(img, 0) {
			b, ok := born[p]
			if !ok || f.Age < 0 || s.facePx(f) < dateMinPx {
				continue
			}
			if b > year+dateUnborn && b > unborn {
				d.Unborn, unborn = s.people[p].Name, b
			}
			// The year this face says, and how sure: children's ages
			// are read to a year or two, adults' much more loosely.
			sd := dateKidAge + dateAgeErr*float64(f.Age)
			wt := 1 / (sd * sd)
			sum += wt * float64(b+f.Age)
			w += wt
			d.Faces = append(d.Faces, DateFace{FaceView: s.faceView(f), Name: s.people[p].Name, Born: b, Looks: f.Age})
		}
		if year+dateUnborn < latest {
			here := s.whoIn(img, 0)
			for _, fid := range s.facesByImage[img] {
				f := s.faces[fid]
				if f == nil || !s.shown(f) || f.Picture || f.Age < 0 || len(f.emb) == 0 || s.facePx(f) < dateMinPx {
					continue
				}
				if c := s.clusters[f.ClusterID]; f.Tagged != 0 || c != nil && (c.PersonID != 0 || c.Hidden) {
					continue
				}
				for pid, rs := range refs {
					b := born[pid]
					if b <= year+dateUnborn || here[pid] != nil {
						continue
					}
					if like, _ := bestRef(unitVec(f.emb), rs); like >= dateLike {
						if b > unborn {
							d.Unborn, unborn = s.people[pid].Name, b
						}
						sd := dateKidAge + dateAgeErr*float64(f.Age)
						wt := 1 / (sd * sd)
						sum += wt * float64(b+f.Age)
						w += wt
						d.Faces = append(d.Faces, DateFace{FaceView: s.faceView(f), Name: s.people[pid].Name, Born: b, Looks: f.Age})
						break
					}
				}
			}
		}
		if w == 0 {
			continue
		}
		est := float32(sum / w)
		d.Err = float32(1 / math.Sqrt(w))
		d.off = abs32(year - est)
		if d.Unborn == "" && (d.off < dateOff || d.off < dateSure*d.Err) {
			continue
		}
		// A copy is made after the original: a photo looking younger than
		// its date is a mistaken face, unless someone wasn't born yet (a
		// camera's clock reset).
		if est > year && d.Unborn == "" {
			continue
		}
		// Faces that disagree among themselves (a photo of a photo, a
		// wrong name) say nothing sure.
		agree := true
		for _, f := range d.Faces {
			sd := dateKidAge + dateAgeErr*f.Looks
			if abs32(f.Born+f.Looks-est) > dateAgree*sd {
				agree = false
			}
		}
		if !agree {
			continue
		}
		d.Year = int(math.Round(float64(est)))
		slices.SortFunc(d.Faces, func(a, b DateFace) int { return cmp.Compare(a.Looks, b.Looks) })
		out = append(out, d)
	}
	// Surest first: someone not born yet, then by how far off for how
	// unsure.
	slices.SortFunc(out, func(a, b DateDoubt) int {
		return cmp.Or(cmp.Compare(boolInt(b.Unborn != ""), boolInt(a.Unborn != "")),
			cmp.Compare(b.off/b.Err, a.off/a.Err), cmp.Compare(a.Image, b.Image))
	})
	if limit > 0 && len(out) > limit {
		out = out[:limit]
	}
	return out
}

// yearOf is a Unix time as a fractional year.
func yearOf(t int64) float32 {
	return 1970 + float32(float64(t)/(365.25*86400))
}

// yearTime is the middle of a year, as Unix seconds: the date set for a
// photo known only by its year.
func yearTime(y int) int64 {
	return time.Date(y, 7, 1, 12, 0, 0, 0, time.UTC).Unix()
}

// dateSession is how many doubtful photos one camera's day must hold to
// be offered as a whole: a session of scanning old prints.
const dateSession = 10

// DateSession is a day of copying old photos (a scanner's, say): the
// photos the faces date, and the others from it, dated together.
type DateSession struct {
	Day    string  `json:"day"`    // the day they carry, YYYY-MM-DD
	Camera string  `json:"camera"` // what made them
	Year   int     `json:"year"`   // the middle year the faces say
	From   int     `json:"from"`   // and the range most fall in
	To     int     `json:"to"`
	Dated  int     `json:"dated"`  // photos the faces date
	Others []int64 `json:"others"` // the rest, no date of their own to go by
	// Photos are the ones the faces date, with the year each.
	Photos []SessionPhoto `json:"photos"`
}

// SessionPhoto is a photo of a DateSession and the year its faces say.
type SessionPhoto struct {
	Image int64 `json:"image"`
	Year  int   `json:"year"`
}

// DateSessions lists days whose photos are mostly far older than they say
// (a session of scanning), with the photos from them the faces can't date.
func (s *Store) DateSessions(ctx context.Context) []DateSession {
	s.mu.RLock()
	defer s.mu.RUnlock()
	type key struct{ day, camera string }
	camera := func(im *imageRec) string { return im.CameraMake + " " + im.CameraModel }
	years := map[key][]int{}
	photos := map[key][]SessionPhoto{}
	doubted := map[int64]bool{}
	for _, d := range s.doubtfulDates(0) {
		im := s.images[d.Image]
		k := key{time.Unix(d.Taken, 0).UTC().Format(time.DateOnly), camera(im)}
		years[k] = append(years[k], d.Year)
		photos[k] = append(photos[k], SessionPhoto{d.Image, d.Year})
		doubted[d.Image] = true
	}
	var out []DateSession
	for k, ys := range years {
		if len(ys) < dateSession {
			continue
		}
		slices.Sort(ys)
		ds := DateSession{Day: k.day, Camera: k.camera, Year: ys[len(ys)/2], From: ys[len(ys)/10], To: ys[len(ys)*9/10], Dated: len(ys), Photos: photos[k]}
		for id, im := range s.images {
			if im.TakenAt == nil || im.UserTaken != nil || im.DateOK || doubted[id] || camera(im) != k.camera ||
				time.Unix(*im.TakenAt, 0).UTC().Format(time.DateOnly) != k.day {
				continue
			}
			ds.Others = append(ds.Others, id)
		}
		slices.Sort(ds.Others)
		out = append(out, ds)
	}
	slices.SortFunc(out, func(a, b DateSession) int { return cmp.Compare(b.Dated, a.Dated) })
	return out
}
