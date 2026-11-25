package domain

import "testing"

func TestKindsAndEmptySections(t *testing.T) {
	sections := EmptySections()
	if len(sections) != 11 {
		t.Fatalf("got %d sections", len(sections))
	}
	for _, kind := range Kinds() {
		if !kind.Valid() || sections[kind] == nil || len(sections[kind]) != 0 {
			t.Errorf("invalid empty section %q", kind)
		}
	}
	for _, kind := range []Kind{"", "Products", "admin", "products/"} {
		if kind.Valid() {
			t.Errorf("accepted %q", kind)
		}
	}
	first := Kinds()
	first[0] = "changed"
	if !Banner.Valid() {
		t.Fatal("caller mutated shared kinds")
	}
}
