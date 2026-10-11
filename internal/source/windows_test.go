package source

import "testing"

func TestParseWindowsPath(t *testing.T) {
	cases := map[string]string{
		`C:\Users\me\Pictures`:  "/C:/Users/me/Pictures",
		`c:/Users/me/Pictures/`: "/C:/Users/me/Pictures",
		`D:\`:                   "/D:",
		`D:`:                    "/D:",
		`file:///E:/Photos`:     "/E:/Photos",
	}
	for in, want := range cases {
		s, err := Parse(in)
		if err != nil {
			t.Errorf("Parse(%q): %v", in, err)
			continue
		}
		if s.Scheme != "file" || s.Path != want {
			t.Errorf("Parse(%q) = %q %q, want file %q", in, s.Scheme, s.Path, want)
		}
	}
	for _, in := range []string{`C`, `CD:\x`, `relative\dir`, `1:\x`} {
		if s, err := Parse(in); err == nil && s.Scheme == "file" && isDrivePath(s.Path) {
			t.Errorf("Parse(%q) accepted as a drive path: %q", in, s.Path)
		}
	}
}

func TestDrivePathRoundTrip(t *testing.T) {
	s, err := Parse(`C:\Photos\2024`)
	if err != nil {
		t.Fatal(err)
	}
	loc := s.Location()
	if loc != `C:\Photos\2024` {
		t.Errorf("Location = %q", loc)
	}
	again, err := Parse(loc)
	if err != nil || again.Path != s.Path {
		t.Errorf("Parse(Location) = %+v, %v; want path %q", again, err, s.Path)
	}
	root, _ := Parse(`C:\`)
	if root.Location() != `C:\` {
		t.Errorf("root Location = %q", root.Location())
	}
	p, ok := root.Parent()
	if !ok || p.Path != "/" {
		t.Errorf("root Parent = %+v, %v", p, ok)
	}
	if !Within(s, root) || Within(root, s) {
		t.Error("Within on drive paths")
	}
	if got := Display(loc, "sub/file.jpg"); got != `C:\Photos\2024\sub\file.jpg` {
		t.Errorf("Display = %q", got)
	}
}
