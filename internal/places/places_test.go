package places

import "testing"

func TestNearest(t *testing.T) {
	if Count() < 100000 {
		t.Fatalf("only %d places loaded", Count())
	}
	cases := []struct {
		lat, lon float64
		name, cc string
	}{
		{55.6761, 12.5683, "Copenhagen", "DK"}, // Rådhuspladsen
		{57.7209, 10.5839, "Skagen", "DK"},
		{41.4036, 2.1744, "Barcelona", "ES"}, // Sagrada Família
		{40.7580, -73.9855, "Manhattan", "US"},
	}
	for _, c := range cases {
		p, ok := Nearest(c.lat, c.lon)
		if !ok || p.Country != c.cc {
			t.Errorf("%v,%v: got %+v %v, want %s in %s", c.lat, c.lon, p, ok, c.name, c.cc)
			continue
		}
		t.Logf("%v,%v → %s (%s)", c.lat, c.lon, p.Name, p.Country)
	}
	// The middle of the North Atlantic: no town near.
	if p, ok := Nearest(45, -35); ok {
		t.Errorf("mid-Atlantic named %+v", p)
	}
	if _, ok := Nearest(100, 0); ok {
		t.Error("an impossible position was named")
	}
}

func TestLabel(t *testing.T) {
	cases := []struct {
		lat, lon float64
		level    Level
		want     string
	}{
		{57.7209, 10.5839, ByArea, "Frederikshavn"},
		{57.7209, 10.5839, ByTown, "Skagen"},
		{55.6761, 12.5683, ByArea, "København"},
		{41.4036, 2.1744, ByArea, "Barcelona, Spanien"},
	}
	for _, c := range cases {
		p, ok := Nearest(c.lat, c.lon)
		if !ok {
			t.Fatalf("%v,%v: no place", c.lat, c.lon)
		}
		if got := p.Label(c.level, "DK"); got != c.want {
			t.Errorf("%v,%v level %d: %q (place %+v), want %q", c.lat, c.lon, c.level, got, p, c.want)
		}
	}
	if got := SafeName(`a/b:c?`); got != "a-b-c-" && got != "a-b-c" {
		t.Errorf("SafeName = %q", got)
	}
}
