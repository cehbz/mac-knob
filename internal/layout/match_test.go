package layout

import "testing"

func TestMatchExactTitleBeatsFrame(t *testing.T) {
	saved := []SavedWindow{
		{BundleID: "com.app", Title: "Doc A", Frame: Rect{0, 0, 800, 600}},
		{BundleID: "com.app", Title: "Doc B", Frame: Rect{900, 0, 800, 600}},
	}
	// Frames swapped relative to save: titles must win.
	live := []LiveWindow{
		{ID: 11, BundleID: "com.app", Title: "Doc B", Frame: Rect{0, 0, 800, 600}},
		{ID: 22, BundleID: "com.app", Title: "Doc A", Frame: Rect{900, 0, 800, 600}},
	}
	m := Match(saved, live)
	if m[0] != 22 || m[1] != 11 {
		t.Fatalf("want {0:22, 1:11}, got %v", m)
	}
}

func TestMatchFallsBackToFrameWhenTitlesChanged(t *testing.T) {
	saved := []SavedWindow{
		{BundleID: "com.browser", Title: "Old tab title", Frame: Rect{0, 0, 1000, 800}},
		{BundleID: "com.browser", Title: "Another old tab", Frame: Rect{2000, 100, 600, 400}},
	}
	live := []LiveWindow{
		{ID: 1, BundleID: "com.browser", Title: "New tab title", Frame: Rect{1990, 105, 600, 400}},
		{ID: 2, BundleID: "com.browser", Title: "Different now", Frame: Rect{5, 0, 1000, 800}},
	}
	m := Match(saved, live)
	if m[0] != 2 || m[1] != 1 {
		t.Fatalf("want {0:2, 1:1}, got %v", m)
	}
}

func TestMatchNeverCrossesApps(t *testing.T) {
	saved := []SavedWindow{
		{BundleID: "com.a", Title: "Same Title", Frame: Rect{0, 0, 100, 100}},
	}
	live := []LiveWindow{
		{ID: 7, BundleID: "com.b", Title: "Same Title", Frame: Rect{0, 0, 100, 100}},
	}
	if m := Match(saved, live); len(m) != 0 {
		t.Fatalf("want no matches across apps, got %v", m)
	}
}

func TestMatchUnmatchedSavedAbsent(t *testing.T) {
	saved := []SavedWindow{
		{BundleID: "com.app", Title: "One", Frame: Rect{0, 0, 100, 100}},
		{BundleID: "com.app", Title: "Two", Frame: Rect{200, 0, 100, 100}},
	}
	live := []LiveWindow{
		{ID: 5, BundleID: "com.app", Title: "One", Frame: Rect{0, 0, 100, 100}},
	}
	m := Match(saved, live)
	if m[0] != 5 {
		t.Fatalf("want 0:5, got %v", m)
	}
	if _, ok := m[1]; ok {
		t.Fatalf("saved[1] has no live counterpart, got %v", m)
	}
}

func TestMatchLiveWindowUsedOnce(t *testing.T) {
	saved := []SavedWindow{
		{BundleID: "com.app", Title: "Same", Frame: Rect{0, 0, 100, 100}},
		{BundleID: "com.app", Title: "Same", Frame: Rect{0, 0, 100, 100}},
	}
	live := []LiveWindow{
		{ID: 9, BundleID: "com.app", Title: "Same", Frame: Rect{0, 0, 100, 100}},
	}
	m := Match(saved, live)
	if len(m) != 1 {
		t.Fatalf("one live window can satisfy only one saved window, got %v", m)
	}
}

func TestMatchOwnerNameFallback(t *testing.T) {
	saved := []SavedWindow{
		{OwnerName: "LegacyApp", Title: "W", Frame: Rect{0, 0, 100, 100}},
	}
	live := []LiveWindow{
		{ID: 3, OwnerName: "LegacyApp", Title: "W", Frame: Rect{0, 0, 100, 100}},
	}
	if m := Match(saved, live); m[0] != 3 {
		t.Fatalf("want 0:3 via owner-name identity, got %v", m)
	}
}

func TestResolveSpacesByUUID(t *testing.T) {
	saved := []SavedSpace{{UUID: "AAA", DisplayUUID: "D1", Index: 0}}
	displays := []CurrentDisplay{
		{UUID: "D1", Spaces: []CurrentSpace{{ID: 101, UUID: "AAA"}, {ID: 102, UUID: "BBB"}}},
	}
	m := ResolveSpaces(saved, displays)
	if m["AAA"] != 101 {
		t.Fatalf("want AAA->101, got %v", m)
	}
}

func TestResolveSpacesUUIDFoundOnOtherDisplay(t *testing.T) {
	// Space migrated displays but kept its UUID: UUID match wins.
	saved := []SavedSpace{{UUID: "AAA", DisplayUUID: "D1", Index: 0}}
	displays := []CurrentDisplay{
		{UUID: "D2", Spaces: []CurrentSpace{{ID: 201, UUID: "AAA"}}},
	}
	if m := ResolveSpaces(saved, displays); m["AAA"] != 201 {
		t.Fatalf("want AAA->201, got %v", m)
	}
}

func TestResolveSpacesFallbackToIndex(t *testing.T) {
	saved := []SavedSpace{{UUID: "GONE", DisplayUUID: "D1", Index: 1}}
	displays := []CurrentDisplay{
		{UUID: "D1", Spaces: []CurrentSpace{{ID: 101, UUID: "AAA"}, {ID: 102, UUID: "BBB"}}},
	}
	if m := ResolveSpaces(saved, displays); m["GONE"] != 102 {
		t.Fatalf("want GONE->102 via (display,index), got %v", m)
	}
}

func TestResolveSpacesMissingDisplayOrIndexSkipped(t *testing.T) {
	saved := []SavedSpace{
		{UUID: "GONE1", DisplayUUID: "DX", Index: 0}, // display gone
		{UUID: "GONE2", DisplayUUID: "D1", Index: 5}, // index out of range
	}
	displays := []CurrentDisplay{
		{UUID: "D1", Spaces: []CurrentSpace{{ID: 101, UUID: "AAA"}}},
	}
	if m := ResolveSpaces(saved, displays); len(m) != 0 {
		t.Fatalf("want no resolution, got %v", m)
	}
}
