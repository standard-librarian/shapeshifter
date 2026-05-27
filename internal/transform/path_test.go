package transform

import "testing"

func TestParseObjectPath(t *testing.T) {
	valid, err := ParseObjectPath(".contact.email")
	if err != nil {
		t.Fatal(err)
	}
	if len(valid) != 2 || valid[0] != "contact" || valid[1] != "email" {
		t.Fatalf("path = %#v", valid)
	}

	for _, path := range []string{".", "", "name", ".contact.", ".contact..email", ".items[0].id", ".kebab-case", ".\"quoted\""} {
		t.Run(path, func(t *testing.T) {
			if _, err := ParseObjectPath(path); err == nil {
				t.Fatal("expected error")
			}
		})
	}
}

func TestSetPathCollision(t *testing.T) {
	target := map[string]any{}
	if err := SetPath(target, []string{"contact"}, "nope"); err != nil {
		t.Fatal(err)
	}
	if err := SetPath(target, []string{"contact", "email"}, "a@example.com"); err == nil {
		t.Fatal("expected collision")
	}
}

func FuzzParseObjectPath(f *testing.F) {
	for _, seed := range []string{".name", ".contact.email", ".", "", "name", ".bad-key"} {
		f.Add(seed)
	}
	f.Fuzz(func(t *testing.T, path string) {
		segments, err := ParseObjectPath(path)
		if err != nil {
			return
		}
		target := map[string]any{}
		if err := SetPath(target, segments, "value"); err != nil {
			t.Fatalf("valid parsed path failed setter: %v", err)
		}
	})
}

func FuzzSetPath(f *testing.F) {
	f.Add("a", "b")
	f.Add("contact", "email")
	f.Fuzz(func(t *testing.T, a, b string) {
		path := "." + a + "." + b
		segments, err := ParseObjectPath(path)
		if err != nil {
			return
		}
		target := map[string]any{}
		if err := SetPath(target, segments, "value"); err != nil {
			t.Fatalf("set failed: %v", err)
		}
	})
}
