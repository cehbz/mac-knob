package layout

import (
	"testing"
	"time"
)

func TestStatsPerDisplay(t *testing.T) {
	l := Layout{
		Spaces: []SavedSpace{
			{UUID: "a", DisplayUUID: "D1", Index: 0},
			{UUID: "b", DisplayUUID: "D1", Index: 1},
			{UUID: "c", DisplayUUID: "D2", Index: 0},
		},
		Windows: make([]SavedWindow, 5),
	}
	st := l.Stats()
	if st.Windows != 5 || st.DisplayCount() != 2 {
		t.Fatalf("windows=%d displays=%d, want 5/2", st.Windows, st.DisplayCount())
	}
	if st.Displays[0].DisplayUUID != "D1" || st.Displays[0].Spaces != 2 {
		t.Fatalf("display0 = %+v, want D1/2", st.Displays[0])
	}
	if st.Displays[1].DisplayUUID != "D2" || st.Displays[1].Spaces != 1 {
		t.Fatalf("display1 = %+v, want D2/1", st.Displays[1])
	}
}

func TestRicherDisplaysBeatWindows(t *testing.T) {
	// Two displays with few windows beats one display packed with windows.
	twoDisp := Layout{Spaces: []SavedSpace{{DisplayUUID: "D1"}, {DisplayUUID: "D2"}}, Windows: make([]SavedWindow, 3)}
	oneDisp := Layout{Spaces: []SavedSpace{{DisplayUUID: "D1"}}, Windows: make([]SavedWindow, 30)}
	if !Richer(twoDisp, oneDisp) {
		t.Fatal("more displays should rank richer than more windows")
	}
}

func TestRicherWindowsThenRecency(t *testing.T) {
	older := Layout{Spaces: []SavedSpace{{DisplayUUID: "D1"}}, Windows: make([]SavedWindow, 10), SavedAt: time.Unix(100, 0)}
	newer := Layout{Spaces: []SavedSpace{{DisplayUUID: "D1"}}, Windows: make([]SavedWindow, 10), SavedAt: time.Unix(200, 0)}
	if !Richer(newer, older) {
		t.Fatal("equal displays+windows should break ties by recency")
	}
	more := Layout{Spaces: []SavedSpace{{DisplayUUID: "D1"}}, Windows: make([]SavedWindow, 11), SavedAt: time.Unix(100, 0)}
	if !Richer(more, newer) {
		t.Fatal("more windows should beat more recent")
	}
}

func TestSignatureOrderIndependent(t *testing.T) {
	a := Layout{
		SavedAt: time.Unix(1, 0),
		Spaces:  []SavedSpace{{UUID: "x", DisplayUUID: "D1", Index: 0}, {UUID: "y", DisplayUUID: "D1", Index: 1}},
		Windows: []SavedWindow{{BundleID: "com.a", Title: "1"}, {BundleID: "com.b", Title: "2"}},
	}
	b := Layout{
		SavedAt: time.Unix(999, 0), // different timestamp must not matter
		Spaces:  []SavedSpace{{UUID: "y", DisplayUUID: "D1", Index: 1}, {UUID: "x", DisplayUUID: "D1", Index: 0}},
		Windows: []SavedWindow{{BundleID: "com.b", Title: "2"}, {BundleID: "com.a", Title: "1"}},
	}
	if a.Signature() != b.Signature() {
		t.Fatal("reordered identical content should share a signature")
	}
}

func TestSignatureDistinguishesContent(t *testing.T) {
	a := Layout{Windows: []SavedWindow{{BundleID: "com.a", Title: "1", SpaceUUID: "s1"}}}
	b := Layout{Windows: []SavedWindow{{BundleID: "com.a", Title: "1", SpaceUUID: "s2"}}}
	if a.Signature() == b.Signature() {
		t.Fatal("different space assignment should change the signature")
	}
}
