// Package layout is the cgo-free domain core of spacekeeper: the saved-layout
// document, window matching, and space resolution. The skylight package is
// kept out so this stays unit-testable.
package layout

import (
	"math"
	"sort"
	"time"
)

type Rect struct {
	X, Y, W, H float64
}

// SavedSpace identifies a space durably: UUID first, position fallback.
// Index is the position within the display's user spaces at save time.
type SavedSpace struct {
	UUID        string `json:"uuid"`
	DisplayUUID string `json:"displayUUID"`
	Index       int    `json:"index"`
}

type SavedWindow struct {
	BundleID  string `json:"bundleID,omitempty"`
	OwnerName string `json:"ownerName"`
	Title     string `json:"title,omitempty"`
	Frame     Rect   `json:"frame"`
	SpaceUUID string `json:"spaceUUID"`
}

type Layout struct {
	SavedAt time.Time     `json:"savedAt"`
	Spaces  []SavedSpace  `json:"spaces"`
	Windows []SavedWindow `json:"windows"`
}

// LiveWindow is a window present right now (CGWindowIDs are session-scoped,
// so live IDs never appear in a Layout).
type LiveWindow struct {
	ID        uint32
	OwnerPID  int
	BundleID  string
	OwnerName string
	Title     string
	Frame     Rect
}

// CurrentSpace / CurrentDisplay mirror the current Mission Control state
// (user spaces only), translated from skylight types by the caller.
type CurrentSpace struct {
	ID   uint64
	UUID string
}

type CurrentDisplay struct {
	UUID   string
	Spaces []CurrentSpace
}

// Match pairs saved windows with live windows. Windows only match within the
// same app (bundle ID, falling back to owner name). Among an app's windows,
// equal titles are the strongest signal, then frame proximity. Each live
// window is used at most once. Returns saved-slice index -> live window ID.
func Match(saved []SavedWindow, live []LiveWindow) map[int]uint32 {
	type pair struct {
		savedIdx, liveIdx int
		score             float64
	}
	var pairs []pair
	for si, s := range saved {
		for li, l := range live {
			if appKey(s.BundleID, s.OwnerName) != appKey(l.BundleID, l.OwnerName) {
				continue
			}
			pairs = append(pairs, pair{si, li, matchScore(s, l)})
		}
	}
	sort.SliceStable(pairs, func(i, j int) bool { return pairs[i].score > pairs[j].score })

	matched := make(map[int]uint32)
	usedSaved := make(map[int]bool)
	usedLive := make(map[int]bool)
	for _, p := range pairs {
		if usedSaved[p.savedIdx] || usedLive[p.liveIdx] {
			continue
		}
		usedSaved[p.savedIdx] = true
		usedLive[p.liveIdx] = true
		matched[p.savedIdx] = live[p.liveIdx].ID
	}
	return matched
}

// appKey gives windows an app identity: bundle ID when known, otherwise the
// owner name (some processes have no bundle).
func appKey(bundleID, ownerName string) string {
	if bundleID != "" {
		return "b:" + bundleID
	}
	return "o:" + ownerName
}

// matchScore rates a saved/live pairing. An exact non-empty title is the
// strongest signal (titles outrank any frame evidence); frame proximity
// breaks ties, since titles drift between sessions (browser tabs, documents).
func matchScore(s SavedWindow, l LiveWindow) float64 {
	score := 0.0
	if s.Title != "" && s.Title == l.Title {
		score += 100
	}
	d := math.Abs(s.Frame.X-l.Frame.X) + math.Abs(s.Frame.Y-l.Frame.Y) +
		math.Abs(s.Frame.W-l.Frame.W) + math.Abs(s.Frame.H-l.Frame.H)
	score += math.Max(0, 50-d/10)
	return score
}

// SpaceDeficits returns, per current display (in the given order), how many
// user spaces the saved layout had beyond what the display has now. Entry i
// aligns with displays[i]; it is 0 when the display has enough (or more)
// spaces, or when the saved layout never referenced that display. Saved
// displays that are no longer present cannot be recreated and are ignored.
func SpaceDeficits(saved []SavedSpace, displays []CurrentDisplay) []int {
	savedPerDisplay := make(map[string]int)
	for _, s := range saved {
		savedPerDisplay[s.DisplayUUID]++
	}
	out := make([]int, len(displays))
	for i, d := range displays {
		if deficit := savedPerDisplay[d.UUID] - len(d.Spaces); deficit > 0 {
			out[i] = deficit
		}
	}
	return out
}

// ResolveSpaces maps each saved space UUID to a current space ID. A space
// whose UUID is gone resolves by (display UUID, index); a saved space whose
// display is gone or whose index is out of range gets no entry.
func ResolveSpaces(saved []SavedSpace, displays []CurrentDisplay) map[string]uint64 {
	byUUID := make(map[string]uint64)
	byDisplay := make(map[string][]CurrentSpace)
	for _, d := range displays {
		byDisplay[d.UUID] = d.Spaces
		for _, sp := range d.Spaces {
			if sp.UUID != "" {
				byUUID[sp.UUID] = sp.ID
			}
		}
	}
	resolved := make(map[string]uint64)
	for _, s := range saved {
		if s.UUID != "" {
			if id, ok := byUUID[s.UUID]; ok {
				resolved[s.UUID] = id
				continue
			}
		}
		if spaces := byDisplay[s.DisplayUUID]; s.Index >= 0 && s.Index < len(spaces) {
			resolved[s.UUID] = spaces[s.Index].ID
		}
	}
	return resolved
}
