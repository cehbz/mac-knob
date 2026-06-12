// spacekeeper saves and restores window-to-space assignments.
//
// Read side is SIP-safe SkyLight introspection. Write side is the
// SIP-enabled bridged move operation (Tahoe 26.4+). Space identity is
// persisted as UUID + (display UUID, index) fallback; windows are
// re-identified across sessions by app + title + frame.
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/cehbz/mac-knob/internal/layout"
	"github.com/cehbz/mac-knob/internal/skylight"
)

func usage() {
	fmt.Fprint(os.Stderr, `usage: spacekeeper <command> [flags]

commands:
  save      snapshot current window-to-space assignments
  restore   move windows back to their saved spaces
  show      print the saved layout

flags (after the command):
  -f path   layout file (default ~/.config/spacekeeper/layout.json)
  -n        restore: dry run, print planned moves without moving
`)
	os.Exit(2)
}

func main() {
	if len(os.Args) < 2 {
		usage()
	}
	cmd := os.Args[1]
	fs := flag.NewFlagSet(cmd, flag.ExitOnError)
	file := fs.String("f", defaultLayoutPath(), "layout file")
	dryRun := fs.Bool("n", false, "dry run")
	fs.Parse(os.Args[2:])

	var err error
	switch cmd {
	case "save":
		err = save(*file)
	case "restore":
		err = restore(*file, *dryRun)
	case "show":
		err = show(*file)
	default:
		usage()
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, "spacekeeper: "+err.Error())
		os.Exit(1)
	}
}

func defaultLayoutPath() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return "layout.json"
	}
	return filepath.Join(home, ".config", "spacekeeper", "layout.json")
}

// spaceKey returns a durable identity for a space. The first desktop
// historically has an empty UUID, so synthesize one from position.
func spaceKey(uuid, displayUUID string, index int) string {
	if uuid != "" {
		return uuid
	}
	return fmt.Sprintf("display:%s/%d", displayUUID, index)
}

// snapshot is the shared read side: current user spaces per display, and
// every normal window with its single user-space assignment.
type snapshot struct {
	spaces   []layout.SavedSpace
	displays []layout.CurrentDisplay
	windows  []layout.LiveWindow
	winSpace map[uint32]uint64 // window ID -> current space ID
	idToKey  map[uint64]string // space ID -> spaceKey
}

func gather() (*snapshot, error) {
	displays, err := skylight.ManagedDisplaySpaces()
	if err != nil {
		return nil, err
	}
	s := &snapshot{
		winSpace: make(map[uint32]uint64),
		idToKey:  make(map[uint64]string),
	}
	for _, d := range displays {
		cur := layout.CurrentDisplay{UUID: d.UUID}
		idx := 0
		for _, sp := range d.Spaces {
			if !sp.UserSpace() {
				continue
			}
			key := spaceKey(sp.UUID, d.UUID, idx)
			s.spaces = append(s.spaces, layout.SavedSpace{UUID: key, DisplayUUID: d.UUID, Index: idx})
			cur.Spaces = append(cur.Spaces, layout.CurrentSpace{ID: sp.ID(), UUID: sp.UUID})
			s.idToKey[sp.ID()] = key
			idx++
		}
		s.displays = append(s.displays, cur)
	}

	wins, err := skylight.WindowList()
	if err != nil {
		return nil, err
	}
	sawTitle := false
	for _, w := range wins {
		if w.Layer != 0 || w.Alpha == 0 || w.Bounds.Width < 50 || w.Bounds.Height < 50 {
			continue
		}
		ids, err := skylight.SpacesForWindow(w.Number)
		if err != nil {
			continue
		}
		var userSpaces []uint64
		for _, id := range ids {
			if _, ok := s.idToKey[id]; ok {
				userSpaces = append(userSpaces, id)
			}
		}
		// Sticky windows (several spaces) and fullscreen windows (zero
		// user spaces) have no single home; skip them.
		if len(userSpaces) != 1 {
			continue
		}
		if w.Name != "" {
			sawTitle = true
		}
		s.windows = append(s.windows, layout.LiveWindow{
			ID:        w.Number,
			BundleID:  skylight.BundleIDForPID(w.OwnerPID),
			OwnerName: w.OwnerName,
			Title:     w.Name,
			Frame:     layout.Rect{X: w.Bounds.X, Y: w.Bounds.Y, W: w.Bounds.Width, H: w.Bounds.Height},
		})
		s.winSpace[w.Number] = userSpaces[0]
	}
	if len(s.windows) > 0 && !sawTitle {
		fmt.Fprintln(os.Stderr, "warning: no window titles visible — grant Screen Recording for reliable matching (System Settings > Privacy & Security > Screen Recording)")
	}
	return s, nil
}

func save(file string) error {
	s, err := gather()
	if err != nil {
		return err
	}
	l := layout.Layout{SavedAt: time.Now()}
	l.Spaces = s.spaces
	for _, w := range s.windows {
		l.Windows = append(l.Windows, layout.SavedWindow{
			BundleID:  w.BundleID,
			OwnerName: w.OwnerName,
			Title:     w.Title,
			Frame:     w.Frame,
			SpaceUUID: s.idToKey[s.winSpace[w.ID]],
		})
	}
	if err := os.MkdirAll(filepath.Dir(file), 0o755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(l, "", "  ")
	if err != nil {
		return err
	}
	if err := os.WriteFile(file, data, 0o600); err != nil {
		return err
	}
	fmt.Printf("saved %d windows across %d spaces to %s\n", len(l.Windows), len(l.Spaces), file)
	return nil
}

func restore(file string, dryRun bool) error {
	data, err := os.ReadFile(file)
	if err != nil {
		return err
	}
	var l layout.Layout
	if err := json.Unmarshal(data, &l); err != nil {
		return err
	}

	s, err := gather()
	if err != nil {
		return err
	}
	resolved := layout.ResolveSpaces(l.Spaces, s.displays)
	matched := layout.Match(l.Windows, s.windows)

	moves := make(map[uint64][]uint32) // target space ID -> window IDs
	skipped, unresolved := 0, 0
	for si, wid := range matched {
		target, ok := resolved[l.Windows[si].SpaceUUID]
		if !ok {
			unresolved++
			continue
		}
		if s.winSpace[wid] == target {
			skipped++
			continue
		}
		if dryRun {
			fmt.Printf("would move %q %q (window %d) -> space %d\n",
				l.Windows[si].OwnerName, l.Windows[si].Title, wid, target)
			continue
		}
		moves[target] = append(moves[target], wid)
	}

	moveCount := 0
	for target, wids := range moves {
		if err := skylight.MoveWindowsToSpace(wids, target); err != nil {
			return fmt.Errorf("moving %d windows to space %d: %w", len(wids), target, err)
		}
		moveCount += len(wids)
	}

	verified := 0
	if moveCount > 0 {
		// The bridged operation is asynchronous; give it a beat, then check.
		time.Sleep(500 * time.Millisecond)
		for target, wids := range moves {
			for _, wid := range wids {
				ids, err := skylight.SpacesForWindow(wid)
				if err == nil && len(ids) == 1 && ids[0] == target {
					verified++
				}
			}
		}
	}

	fmt.Printf("matched %d/%d saved windows; moved %d (%d verified), %d already in place",
		len(matched), len(l.Windows), moveCount, verified, skipped)
	if unresolved > 0 {
		fmt.Printf(", %d on spaces that no longer exist", unresolved)
	}
	fmt.Println()
	if moveCount > 0 && verified < moveCount {
		fmt.Fprintln(os.Stderr, "warning: some moves did not verify — the bridged-move API may be restricted on this macOS build")
	}
	return nil
}

func show(file string) error {
	data, err := os.ReadFile(file)
	if err != nil {
		return err
	}
	_, err = os.Stdout.Write(append(data, '\n'))
	return err
}
