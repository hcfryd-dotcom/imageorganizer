package server

import (
	"cmp"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"slices"
	"strconv"
	"strings"

	"github.com/lkarlslund/imageorganizer/internal/store"
)

// People endpoints. A person is made of match groups: name an unknown
// group, then confirm or reject "Is this also <name>?" candidates.

func (s *Server) peopleRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/people", s.listPeople)
	mux.HandleFunc("PATCH /api/people/{id}", s.updatePerson)
	mux.HandleFunc("GET /api/people/{id}/groups", s.personGroups)
	mux.HandleFunc("GET /api/people/{id}/faces", s.personFaces)
	mux.HandleFunc("GET /api/people/{id}/candidates", s.personCandidates)
	mux.HandleFunc("GET /api/people/{id}/face-candidates", s.personFaceCandidates)
	mux.HandleFunc("GET /api/people/{id}/doubts", s.personDoubts)
	mux.HandleFunc("GET /api/people/{id}/pictures", s.personPictures)
	mux.HandleFunc("POST /api/people/{id}/pictures", s.setPersonPictures)
	mux.HandleFunc("POST /api/people/{id}/merge", s.mergePerson)
	mux.HandleFunc("POST /api/people/{id}/relations", s.setRelation)
	mux.HandleFunc("GET /api/groups/{id}/faces", s.groupFaces)
	mux.HandleFunc("POST /api/groups/{id}/name", s.nameGroup)
	mux.HandleFunc("POST /api/groups/{id}/reject", s.rejectGroup)
	mux.HandleFunc("POST /api/groups/{id}/hide", s.hideGroup)
	mux.HandleFunc("GET /api/images/{id}/event", s.eventStrangers)
	mux.HandleFunc("POST /api/images/{id}/event/strangers", s.hideEventStrangers)
	mux.HandleFunc("POST /api/groups/unhide", s.unhideGroups)
	mux.HandleFunc("POST /api/groups/{id}/notface", s.notFacesInGroup)
	mux.HandleFunc("POST /api/groups/{id}/restore", s.restoreGroupFaces)
	mux.HandleFunc("POST /api/faces/{id}/remove", s.removeFace)
	mux.HandleFunc("POST /api/faces/{id}/reject", s.rejectFace)
	mux.HandleFunc("POST /api/faces/{id}/skip", s.skipFace)
	mux.HandleFunc("POST /api/faces/{id}/notface", s.notAFace)
	mux.HandleFunc("POST /api/faces/{id}/picture", s.facePicture)
	mux.HandleFunc("POST /api/faces/{id}/tag", s.tagFace)
	mux.HandleFunc("GET /api/faces/{id}/suggestions", s.faceSuggestions)
	mux.HandleFunc("GET /api/faces/{id}/why", s.explainFace)
	mux.HandleFunc("GET /api/images/{id}/why", s.explainImage)
	mux.HandleFunc("GET /api/groups/{id}", s.groupItem)
	mux.HandleFunc("GET /api/images/{id}/faces", s.imageFaces)
	mux.HandleFunc("GET /api/review", s.review)
	mux.HandleFunc("GET /api/matches", s.faceMatches)
	mux.HandleFunc("GET /api/lanes/people", s.lanePeople)
	mux.HandleFunc("GET /api/discover", s.discoverPeople)
	mux.HandleFunc("POST /api/discover/name", s.nameDiscovered)
	mux.HandleFunc("GET /api/discover/queue", s.discoverQueue)
	mux.HandleFunc("POST /api/discover/hide", s.hideDiscovered)
	mux.HandleFunc("POST /api/discover/reject", s.rejectDiscovered)
	mux.HandleFunc("GET /api/discover/relations", s.guessRelations)
	mux.HandleFunc("POST /api/discover/relations", s.answerRelation)
	mux.HandleFunc("POST /api/discover/shot", s.answerShot)
	mux.HandleFunc("GET /api/discover/dates", s.doubtfulDates)
	mux.HandleFunc("GET /api/discover/faces", s.doubtfulFaces)
	mux.HandleFunc("POST /api/lanes/{id}/auto", s.laneAuto)
	mux.HandleFunc("POST /api/faces/{id}/unaccept", s.unacceptFace)
	mux.HandleFunc("POST /api/groups/{id}/skip", s.skipGroup)
	mux.HandleFunc("GET /api/groups/{id}/suggestions", s.groupSuggestions)
}

func (s *Server) listPeople(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, map[string]any{
		"people":         s.Store.People(r.Context()),
		"review":         s.Store.ReviewCount(s.Store.ReviewMin()),
		"face_photos":    s.Store.FacePhotoCounts(), // by filter: "known", "one-unknown", ...
		"alone_photos":   len(s.Store.AloneImageIDs()),
		"picture_photos": len(s.Store.PictureImageIDs(0)),
	})
}

func (s *Server) review(w http.ResponseWriter, r *http.Request) {
	offset, _ := strconv.Atoi(r.URL.Query().Get("offset"))
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	if limit <= 0 || limit > 200 {
		limit = 40
	}
	items, total := s.Store.Review(r.Context(), s.Store.ReviewMin(), offset, limit, 8, r.URL.Query().Get("order"))
	writeJSON(w, map[string]any{"total": total, "faces": s.Store.ReviewFaces(s.Store.ReviewMin()), "items": items})
}

func (s *Server) groupSuggestions(w http.ResponseWriter, r *http.Request) {
	id, err := pathID(r)
	if err != nil {
		httpError(w, 400, err)
		return
	}
	writeJSON(w, s.Store.GroupSuggestions(r.Context(), id))
}

func (s *Server) skipGroup(w http.ResponseWriter, r *http.Request) {
	id, err := pathID(r)
	if err != nil {
		httpError(w, 400, err)
		return
	}
	if err := s.Store.SkipGroup(r.Context(), id); err != nil {
		storeError(w, err)
		return
	}
	writeJSON(w, map[string]bool{"ok": true})
}

func (s *Server) updatePerson(w http.ResponseWriter, r *http.Request) {
	id, err := pathID(r)
	if err != nil {
		httpError(w, 400, err)
		return
	}
	var req store.PersonEdit
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httpError(w, 400, err)
		return
	}
	if err := s.Store.UpdatePerson(r.Context(), id, req); err != nil {
		storeError(w, err)
		return
	}
	writeJSON(w, map[string]bool{"ok": true})
}

func (s *Server) personGroups(w http.ResponseWriter, r *http.Request) {
	id, err := pathID(r)
	if err != nil {
		httpError(w, 400, err)
		return
	}
	writeJSON(w, nonNil(s.Store.PersonGroups(r.Context(), id, 8)))
}

// personFaces lists a person's faces, confirmed and filed, newest first.
func (s *Server) personFaces(w http.ResponseWriter, r *http.Request) {
	id, err := pathID(r)
	if err != nil {
		httpError(w, 400, err)
		return
	}
	writeJSON(w, nonNil(s.Store.PersonFaces(r.Context(), id)))
}

func (s *Server) personCandidates(w http.ResponseWriter, r *http.Request) {
	id, err := pathID(r)
	if err != nil {
		httpError(w, 400, err)
		return
	}
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	if limit <= 0 {
		limit = 12
	}
	writeJSON(w, nonNil(s.Store.Candidates(r.Context(), id, limit, 6)))
}

// personPictures finds pictures of the person (on a wall, say): faces of
// them at an age they had long passed.
func (s *Server) personPictures(w http.ResponseWriter, r *http.Request) {
	id, err := pathID(r)
	if err != nil {
		httpError(w, 400, err)
		return
	}
	writeJSON(w, nonNil(s.Store.FrozenFor(r.Context(), id)))
}

// setPersonPictures answers "a picture of them?" for faces.
func (s *Server) setPersonPictures(w http.ResponseWriter, r *http.Request) {
	id, err := pathID(r)
	if err != nil {
		httpError(w, 400, err)
		return
	}
	var req struct {
		Faces   []int64 `json:"faces"`
		Picture bool    `json:"picture"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httpError(w, 400, err)
		return
	}
	if err := s.Store.SetFrozen(r.Context(), id, req.Faces, req.Picture); err != nil {
		storeError(w, err)
		return
	}
	writeJSON(w, map[string]bool{"ok": true})
}

// personFaceCandidates lists single faces that may be the person.
func (s *Server) personFaceCandidates(w http.ResponseWriter, r *http.Request) {
	id, err := pathID(r)
	if err != nil {
		httpError(w, 400, err)
		return
	}
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	writeJSON(w, nonNil(s.Store.FaceCandidates(r.Context(), id, min(max(limit, 1), 200))))
}

// personDoubts is "Is this really …?" on a person's page: their faces,
// the least likely them first.
func (s *Server) personDoubts(w http.ResponseWriter, r *http.Request) {
	id, err := pathID(r)
	if err != nil {
		httpError(w, 400, err)
		return
	}
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	writeJSON(w, nonNil(s.Store.PersonDoubts(r.Context(), id, min(max(limit, 1), 200))))
}

// faceMatches lists unknown faces by how surely they are someone named,
// for the one-face-at-a-time matcher.
func (s *Server) faceMatches(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	limit, _ := strconv.Atoi(q.Get("limit"))
	minAge, _ := strconv.ParseFloat(q.Get("min_age"), 32)
	maxAge, _ := strconv.ParseFloat(q.Get("max_age"), 32)
	o := store.MatchOptions{MinAge: float32(minAge), MaxAge: float32(maxAge), Sort: q.Get("sort")}
	writeJSON(w, nonNil(s.Store.FaceMatches(r.Context(), min(max(limit, 1), 200), o)))
}

// discoverPeople lists the unnamed people who matter most (experiment).
func (s *Server) discoverPeople(w http.ResponseWriter, r *http.Request) {
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	writeJSON(w, nonNil(s.Store.DiscoverPeople(r.Context(), min(max(limit, 1), 100))))
}

// nameDiscovered names a person Discover asks about, or files more faces
// under someone: the faces the user saw and kept ticked are them (yes),
// the ones ticked off are not (no). Nothing unseen is named.
func (s *Server) nameDiscovered(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Name   string  `json:"name"`
		Person int64   `json:"person"` // or who they are, by id
		Yes    []int64 `json:"yes"`
		No     []int64 `json:"no"`
		// Picture: it's a painting, statue or poster, named (it never
		// ages, and doesn't count as someone in the photo).
		Picture bool `json:"picture"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || len(req.Yes)+len(req.No) == 0 {
		httpError(w, 400, fmt.Errorf("faces and a name required"))
		return
	}
	known := s.Store.KnownPerson(req.Person, req.Name)
	pid, err := s.Store.ConfirmFaces(r.Context(), req.Person, req.Name, req.Yes, req.No)
	if err != nil {
		storeError(w, err)
		return
	}
	switch {
	case req.Picture && known:
		// Someone real (named before): these faces are pictures of them,
		// a poster, a portrait. They themselves still age.
		for _, id := range req.Yes {
			if err := s.Store.SetFacePicture(r.Context(), id, true); err != nil {
				storeError(w, err)
				return
			}
		}
	case req.Picture:
		// A new name for a painting, a statue: it never ages.
		if err := s.Store.UpdatePerson(r.Context(), pid, store.PersonEdit{Picture: &req.Picture}); err != nil {
			storeError(w, err)
			return
		}
	}
	writeJSON(w, map[string]int64{"person_id": pid})
}

// minMatch is the queue with only matches of at least lo: the faces of
// "More of …?" batches, and the name "Who is this?" suggests. The queue
// as built is shared, so changed items are copies.
func minMatch(all []store.QueueItem, lo float32) []store.QueueItem {
	out := make([]store.QueueItem, 0, len(all))
	for _, it := range all {
		switch {
		case it.More != nil:
			b := *it.More
			b.Faces = slices.DeleteFunc(slices.Clone(b.Faces), func(f store.FaceView) bool { return f.Score < lo })
			if len(b.Faces) == 0 {
				continue
			}
			it.More = &b
		case it.Who != nil && it.Who.Maybe != 0 && it.Who.MaybeLike > 0 && it.Who.MaybeLike < lo:
			ch := *it.Who
			ch.Maybe, ch.MaybeName, ch.MaybeLike = 0, "", 0
			it.Who = &ch
		}
		out = append(out, it)
	}
	return out
}

// ageRange is the queue with only faces looking lo to hi years old (hi 0:
// no upper bound); a face whose age can't be read stays. "Who is this?"
// goes by their age in the last photos of them, a batch keeps the faces
// in range, a doubt or a pair goes by its faces.
func ageRange(all []store.QueueItem, lo, hi float32) []store.QueueItem {
	in := func(age float32) bool { return age < 0 || age >= lo && (hi <= 0 || age <= hi) }
	out := make([]store.QueueItem, 0, len(all))
	for _, it := range all {
		switch {
		case it.Who != nil && !in(it.Who.LastAge):
			continue
		case it.More != nil:
			b := *it.More
			b.Faces = slices.DeleteFunc(slices.Clone(b.Faces), func(f store.FaceView) bool { return !in(f.Age) })
			if len(b.Faces) == 0 {
				continue
			}
			it.More = &b
		case it.Face != nil && !in(it.Face.Face.Age):
			continue
		case it.Shot != nil && !in(it.Shot.A.Age) && !in(it.Shot.B.Age):
			continue
		}
		out = append(out, it)
	}
	return out
}

// whoOrder are the orders "Who is this?" can be shown in, most first
// (else: by worth, as in the whole queue).
var whoOrder = map[string]func(*store.Chain) int{
	"photos": func(c *store.Chain) int { return c.Photos },
	"days":   func(c *store.Chain) int { return c.Days },
	"family": func(c *store.Chain) int { return c.TopDays },
	"recent": func(c *store.Chain) int { return c.To },
}

// discoverQueue lists Discover's questions, the most worth first.
func (s *Server) discoverQueue(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	offset, _ := strconv.Atoi(q.Get("offset"))
	limit, _ := strconv.Atoi(q.Get("limit"))
	all := s.discoverItems(true)
	// "Who is this?" without children (kids=hide), and in another order
	// (who_sort): asked for on its tab.
	// An age range (age_min, age_max: how old the faces look): who and
	// which faces are asked about.
	ageMin, _ := strconv.ParseFloat(q.Get("age_min"), 32)
	ageMax, _ := strconv.ParseFloat(q.Get("age_max"), 32)
	if ageMin > 0 || ageMax > 0 {
		all = ageRange(all, float32(ageMin), float32(ageMax))
	}
	// A minimum match (min, 0–1): faces offered under it are left out, a
	// batch left empty goes, and a suggested name under it isn't offered.
	if lo, err := strconv.ParseFloat(q.Get("min"), 32); err == nil && lo > 0 {
		all = minMatch(all, float32(lo))
	}
	if by := whoOrder[q.Get("who_sort")]; by != nil {
		// The "Who is this?" questions reordered among their own places.
		all = slices.Clone(all)
		var at []int
		var who []store.QueueItem
		for i, it := range all {
			if it.Who != nil {
				at, who = append(at, i), append(who, it)
			}
		}
		slices.SortStableFunc(who, func(a, b store.QueueItem) int { return cmp.Compare(by(b.Who), by(a.Who)) })
		for k, i := range at {
			all[i] = who[k]
		}
	}
	// How many of each kind, and just the kinds asked for (kind=who,more).
	kinds := map[string]int{}
	for _, it := range all {
		kinds[it.Kind]++
	}
	items := all
	if k := q.Get("kind"); k != "" {
		want := strings.Split(k, ",")
		items = nil
		for _, it := range all {
			if slices.Contains(want, it.Kind) {
				items = append(items, it)
			}
		}
	}
	offset = min(max(offset, 0), len(items))
	if limit <= 0 {
		limit = 30
	}
	writeJSON(w, map[string]any{"total": len(items), "all": len(all), "kinds": kinds, "items": nonNil(items[offset:min(len(items), offset+limit)])})
}

// hideDiscovered hides a discovered person as a stranger.
func (s *Server) hideDiscovered(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Groups []int64 `json:"groups"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httpError(w, 400, err)
		return
	}
	for _, g := range req.Groups {
		if err := s.Store.HideGroup(r.Context(), g, true); err != nil {
			storeError(w, err)
			return
		}
	}
	writeJSON(w, map[string]bool{"ok": true})
}

// rejectDiscovered answers "No" to "Is this also …?": the groups aren't
// that person.
func (s *Server) rejectDiscovered(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Groups []int64 `json:"groups"`
		Person int64   `json:"person"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.Person == 0 {
		httpError(w, 400, fmt.Errorf("groups and a person required"))
		return
	}
	for _, g := range req.Groups {
		if err := s.Store.RejectGroup(r.Context(), g, req.Person); err != nil {
			storeError(w, err)
			return
		}
	}
	writeJSON(w, map[string]bool{"ok": true})
}

// guessRelations lists pairs of people who are likely family, to ask
// about (experiment).
func (s *Server) guessRelations(w http.ResponseWriter, r *http.Request) {
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	writeJSON(w, nonNil(s.Store.GuessRelations(r.Context(), min(max(limit, 1), 100))))
}

// doubtfulDates lists photos whose date the faces in them disagree with,
// and days of copying old photos (scanning) to date as a whole.
func (s *Server) doubtfulDates(w http.ResponseWriter, r *http.Request) {
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	writeJSON(w, map[string]any{
		"photos":   nonNil(s.Store.DoubtfulDates(r.Context(), min(max(limit, 1), 500))),
		"sessions": nonNil(s.Store.DateSessions(r.Context())),
	})
}

// doubtfulFaces lists faces filed under people that don't fit them: the
// wrong age, unlike the rest, or more like someone else.
func (s *Server) doubtfulFaces(w http.ResponseWriter, r *http.Request) {
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	writeJSON(w, nonNil(s.Store.DoubtfulFaces(r.Context(), min(max(limit, 1), 200))))
}

// answerRelation answers a relation question: kind is what b is to a
// (parent, child, sibling, partner), or anything else ("none",
// "grandparent", …) to just not ask again.
func (s *Server) answerRelation(w http.ResponseWriter, r *http.Request) {
	var req struct {
		A    int64  `json:"a"`
		B    int64  `json:"b"`
		Kind string `json:"kind"`
		Was  string `json:"was"` // what B was entered as, when putting that right
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httpError(w, 400, err)
		return
	}
	var err error
	if req.Was != "" && req.Kind != "skip" && req.Kind != req.Was {
		// Put right: the relation entered goes, whatever replaces it.
		if err := s.Store.SetRelation(r.Context(), req.A, req.B, req.Was, false); err != nil {
			storeError(w, err)
			return
		}
	}
	switch req.Kind {
	case store.KinParent, store.KinChild, store.KinSibling, store.KinPartner, store.KinFriend:
		err = s.Store.SetRelation(r.Context(), req.A, req.B, req.Kind, true)
	default:
		err = s.Store.SetAsked(r.Context(), req.A, req.B, true)
	}
	if err != nil {
		storeError(w, err)
		return
	}
	writeJSON(w, map[string]bool{"ok": true})
}

// lanePeople picks whose Lane Match lanes to show first.
func (s *Server) lanePeople(w http.ResponseWriter, r *http.Request) {
	n, _ := strconv.Atoi(r.URL.Query().Get("n"))
	writeJSON(w, nonNil(s.Store.LanePeople(r.Context(), min(max(n, 1), 30))))
}

// laneAuto files the faces now near certain a person's by themselves.
func (s *Server) laneAuto(w http.ResponseWriter, r *http.Request) {
	id, err := pathID(r)
	if err != nil {
		httpError(w, 400, err)
		return
	}
	acc, err := s.Store.AutoAccept(r.Context(), id)
	if err != nil {
		storeError(w, err)
		return
	}
	writeJSON(w, map[string]any{"accepted": nonNil(acc)})
}

// unacceptFace undoes an automatic accept.
func (s *Server) unacceptFace(w http.ResponseWriter, r *http.Request) {
	id, err := pathID(r)
	if err != nil {
		httpError(w, 400, err)
		return
	}
	var req struct {
		From int64 `json:"from"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httpError(w, 400, err)
		return
	}
	if err := s.Store.UndoAccept(r.Context(), id, req.From); err != nil {
		storeError(w, err)
		return
	}
	writeJSON(w, map[string]bool{"ok": true})
}

// skipFace puts a face off in the matcher.
func (s *Server) skipFace(w http.ResponseWriter, r *http.Request) {
	id, err := pathID(r)
	if err != nil {
		httpError(w, 400, err)
		return
	}
	if err := s.Store.SkipFace(r.Context(), id); err != nil {
		storeError(w, err)
		return
	}
	writeJSON(w, map[string]bool{"ok": true})
}

// rejectFace says one face is not a person.
func (s *Server) rejectFace(w http.ResponseWriter, r *http.Request) {
	id, err := pathID(r)
	if err != nil {
		httpError(w, 400, err)
		return
	}
	var req struct {
		PersonID int64 `json:"person_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.PersonID == 0 {
		httpError(w, 400, fmt.Errorf("person_id required"))
		return
	}
	if err := s.Store.RejectFace(r.Context(), id, req.PersonID); err != nil {
		storeError(w, err)
		return
	}
	writeJSON(w, map[string]bool{"ok": true})
}

// faceSuggestions lists who one face may be, for the photo viewer.
func (s *Server) faceSuggestions(w http.ResponseWriter, r *http.Request) {
	id, err := pathID(r)
	if err != nil {
		httpError(w, 400, err)
		return
	}
	writeJSON(w, s.Store.FaceSuggestions(r.Context(), id))
}

// explainFace is everything that decides who a face is: its group, who
// else is in the photo and how surely, and the match with the likeliest
// people (and ?person=1,2) with why each is or isn't filed by itself.
func (s *Server) explainFace(w http.ResponseWriter, r *http.Request) {
	id, err := pathID(r)
	if err != nil {
		httpError(w, 400, err)
		return
	}
	var ask []int64
	for v := range strings.SplitSeq(r.URL.Query().Get("person"), ",") {
		if pid, err := strconv.ParseInt(v, 10, 64); err == nil {
			ask = append(ask, pid)
		}
	}
	why, err := s.Store.ExplainFace(r.Context(), id, ask)
	if err != nil {
		httpError(w, 404, err)
		return
	}
	writeJSON(w, why)
}

// explainImage is explainFace for each face in a photo.
func (s *Server) explainImage(w http.ResponseWriter, r *http.Request) {
	id, err := pathID(r)
	if err != nil {
		httpError(w, 400, err)
		return
	}
	why, err := s.Store.ExplainImage(r.Context(), id)
	if err != nil {
		httpError(w, 404, err)
		return
	}
	writeJSON(w, why)
}

// setRelation records (or with on: false removes) that person_id is the
// person's parent, child, sibling or partner.
func (s *Server) setRelation(w http.ResponseWriter, r *http.Request) {
	id, err := pathID(r)
	if err != nil {
		httpError(w, 400, err)
		return
	}
	var req struct {
		PersonID int64  `json:"person_id"`
		Kind     string `json:"kind"`
		On       bool   `json:"on"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httpError(w, 400, err)
		return
	}
	if err := s.Store.SetRelation(r.Context(), id, req.PersonID, req.Kind, req.On); err != nil {
		storeError(w, err)
		return
	}
	writeJSON(w, map[string]bool{"ok": true})
}

func (s *Server) mergePerson(w http.ResponseWriter, r *http.Request) {
	id, err := pathID(r)
	if err != nil {
		httpError(w, 400, err)
		return
	}
	var req struct {
		Into int64 `json:"into"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httpError(w, 400, err)
		return
	}
	if err := s.Store.MergePeople(r.Context(), id, req.Into); err != nil {
		storeError(w, err)
		return
	}
	writeJSON(w, map[string]bool{"ok": true})
}

func (s *Server) groupFaces(w http.ResponseWriter, r *http.Request) {
	id, err := pathID(r)
	if err != nil {
		httpError(w, 400, err)
		return
	}
	writeJSON(w, nonNil(s.Store.GroupFaces(r.Context(), id)))
}

// nameGroup assigns a group to a person (new or existing) and returns the
// person with the next "Is this also …?" candidates.
func (s *Server) nameGroup(w http.ResponseWriter, r *http.Request) {
	id, err := pathID(r)
	if err != nil {
		httpError(w, 400, err)
		return
	}
	var req struct {
		Name     string  `json:"name"`
		PersonID int64   `json:"person_id"`
		Shown    []int64 `json:"shown"` // the faces the user saw when deciding
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httpError(w, 400, err)
		return
	}
	pid, err := s.Store.NameGroup(r.Context(), id, req.PersonID, req.Name)
	if err != nil {
		storeError(w, err)
		return
	}
	if err := s.Store.MarkSeen(r.Context(), pid, req.Shown); err != nil {
		s.Log.Warn("recording seen faces failed", "err", err)
	}
	writeJSON(w, map[string]any{
		"person_id":  pid,
		"candidates": nonNil(s.Store.Candidates(r.Context(), pid, 12, 6)),
	})
}

func (s *Server) rejectGroup(w http.ResponseWriter, r *http.Request) {
	id, err := pathID(r)
	if err != nil {
		httpError(w, 400, err)
		return
	}
	var req struct {
		PersonID int64 `json:"person_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.PersonID == 0 {
		httpError(w, 400, errors.New("person_id required"))
		return
	}
	if err := s.Store.RejectGroup(r.Context(), id, req.PersonID); err != nil {
		storeError(w, err)
		return
	}
	writeJSON(w, map[string]bool{"ok": true})
}

func (s *Server) hideGroup(w http.ResponseWriter, r *http.Request) {
	id, err := pathID(r)
	if err != nil {
		httpError(w, 400, err)
		return
	}
	hidden := r.URL.Query().Get("unhide") == ""
	if err := s.Store.HideGroup(r.Context(), id, hidden); err != nil {
		storeError(w, err)
		return
	}
	writeJSON(w, map[string]bool{"ok": true})
}

// eventStrangers previews hiding everyone unnamed at a photo's event.
func (s *Server) eventStrangers(w http.ResponseWriter, r *http.Request) {
	id, err := pathID(r)
	if err != nil {
		httpError(w, 400, err)
		return
	}
	ev, err := s.Store.EventStrangers(r.Context(), id, r.URL.Query().Has("details"))
	if err != nil {
		storeError(w, err)
		return
	}
	writeJSON(w, ev)
}

// hideEventStrangers hides everyone unnamed at a photo's event.
func (s *Server) hideEventStrangers(w http.ResponseWriter, r *http.Request) {
	id, err := pathID(r)
	if err != nil {
		httpError(w, 400, err)
		return
	}
	var req struct {
		SkipImages []int64 `json:"skip_images"`
		KeepFaces  []int64 `json:"keep_faces"`
	}
	if r.ContentLength != 0 {
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			httpError(w, 400, err)
			return
		}
	}
	ev, err := s.Store.HideEventStrangers(r.Context(), id, req.SkipImages, req.KeepFaces)
	if err != nil {
		storeError(w, err)
		return
	}
	writeJSON(w, ev)
}

// unhideGroups shows hidden groups again (an undo).
func (s *Server) unhideGroups(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Groups []int64 `json:"groups"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httpError(w, 400, err)
		return
	}
	if err := s.Store.UnhideGroups(r.Context(), req.Groups); err != nil {
		storeError(w, err)
		return
	}
	writeJSON(w, map[string]bool{"ok": true})
}

// notFacesInGroup says a whole group is not people; it returns the faces'
// ids, for an undo.
func (s *Server) notFacesInGroup(w http.ResponseWriter, r *http.Request) {
	id, err := pathID(r)
	if err != nil {
		httpError(w, 400, err)
		return
	}
	ids, err := s.Store.NotFacesInGroup(r.Context(), id)
	if err != nil {
		storeError(w, err)
		return
	}
	writeJSON(w, map[string]any{"faces": ids})
}

// restoreGroupFaces undoes notFacesInGroup.
func (s *Server) restoreGroupFaces(w http.ResponseWriter, r *http.Request) {
	id, err := pathID(r)
	if err != nil {
		httpError(w, 400, err)
		return
	}
	var req struct {
		Faces []int64 `json:"faces"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httpError(w, 400, err)
		return
	}
	if err := s.Store.RestoreGroupFaces(r.Context(), id, req.Faces); err != nil {
		storeError(w, err)
		return
	}
	writeJSON(w, map[string]bool{"ok": true})
}

func (s *Server) removeFace(w http.ResponseWriter, r *http.Request) {
	id, err := pathID(r)
	if err != nil {
		httpError(w, 400, err)
		return
	}
	if err := s.Store.RemoveFace(r.Context(), id); err != nil {
		storeError(w, err)
		return
	}
	writeJSON(w, map[string]bool{"ok": true})
}

// tagFace tags one face as a person and returns the person with the next
// "Is this also …?" candidates (the face's old group is usually first).
func (s *Server) tagFace(w http.ResponseWriter, r *http.Request) {
	id, err := pathID(r)
	if err != nil {
		httpError(w, 400, err)
		return
	}
	var req struct {
		Name     string `json:"name"`
		PersonID int64  `json:"person_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httpError(w, 400, err)
		return
	}
	pid, err := s.Store.TagFace(r.Context(), id, req.PersonID, req.Name)
	if err != nil {
		storeError(w, err)
		return
	}
	writeJSON(w, map[string]any{
		"person_id":  pid,
		"candidates": []any{}, // whole groups aren't asked about: Discover asks for more of them
	})
}

// facePicture marks a face as a picture of someone (on: false takes it
// off). With person_id, the face is tagged as them first: "a picture of
// Bob".
func (s *Server) facePicture(w http.ResponseWriter, r *http.Request) {
	id, err := pathID(r)
	if err != nil {
		httpError(w, 400, err)
		return
	}
	var req struct {
		On       bool  `json:"on"`
		PersonID int64 `json:"person_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httpError(w, 400, err)
		return
	}
	if req.PersonID != 0 {
		if _, err := s.Store.TagFace(r.Context(), id, req.PersonID, ""); err != nil {
			storeError(w, err)
			return
		}
	}
	if err := s.Store.SetFacePicture(r.Context(), id, req.On); err != nil {
		storeError(w, err)
		return
	}
	writeJSON(w, map[string]bool{"ok": true})
}

func (s *Server) notAFace(w http.ResponseWriter, r *http.Request) {
	id, err := pathID(r)
	if err != nil {
		httpError(w, 400, err)
		return
	}
	if err := s.Store.NotAFace(r.Context(), id); err != nil {
		storeError(w, err)
		return
	}
	writeJSON(w, map[string]bool{"ok": true})
}

// groupItem is one group as a review card, with more sample faces than
// the inbox shows.
func (s *Server) groupItem(w http.ResponseWriter, r *http.Request) {
	id, err := pathID(r)
	if err != nil {
		httpError(w, 400, err)
		return
	}
	// n: how many of its faces to send (to show them all, up to 1000).
	n := 24
	if v, err := strconv.Atoi(r.URL.Query().Get("n")); err == nil && v > 0 {
		n = min(v, 1000)
	}
	it, err := s.Store.GroupItem(r.Context(), id, n)
	if err != nil {
		storeError(w, err)
		return
	}
	writeJSON(w, it)
}

func (s *Server) imageFaces(w http.ResponseWriter, r *http.Request) {
	id, err := pathID(r)
	if err != nil {
		httpError(w, 400, err)
		return
	}
	writeJSON(w, nonNil(s.Store.ImageFaces(r.Context(), id)))
}

func storeError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, store.ErrNotFound):
		httpError(w, 404, err)
	case errors.Is(err, store.ErrNameRequired), errors.Is(err, store.ErrBadRelation):
		httpError(w, 400, err)
	default:
		httpError(w, 500, err)
	}
}

func nonNil[T any](v []T) []T {
	if v == nil {
		return []T{}
	}
	return v
}

// answerShot settles "Same person?": faces a and b, at one place in
// near-identical shots, are both person (or, person 0, two people).
func (s *Server) answerShot(w http.ResponseWriter, r *http.Request) {
	var req struct {
		A      int64 `json:"a"`
		B      int64 `json:"b"`
		Person int64 `json:"person"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httpError(w, 400, err)
		return
	}
	if err := s.Store.AnswerShot(r.Context(), req.A, req.B, req.Person); err != nil {
		storeError(w, err)
		return
	}
	writeJSON(w, map[string]any{"ok": true})
}
