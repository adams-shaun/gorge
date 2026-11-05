package main

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"slices"

	"github.com/adams-shaun/gorge/draft/draftlab"
	"github.com/adams-shaun/gorge/draft/limited/setdata"
	"github.com/adams-shaun/gorge/engine/deck"
)

// seats and rounds are the pod shape: eight seats, three packs each.
const (
	seats  = 8
	rounds = 3
)

// indexEntry is one line of decks.jsonl. A struct, so the key order is
// fixed.
type indexEntry struct {
	ID      string `json:"id"`
	File    string `json:"file"`
	Set     string `json:"set"`
	Seed    uint64 `json:"seed"`
	Seat    int    `json:"seat"`
	DraftID string `json:"draft_id"`
	Drafter string `json:"drafter"`
	Builder string `json:"builder"`
	meta
}

// run drafts o.pods pods and writes their decks, the index and, last, the
// manifest under o.out.
func run(o options, stdout io.Writer) error {
	if err := checkOutEmpty(o.out); err != nil {
		return err
	}
	lab, err := draftlab.Open(o.corpus, draftlab.Options{})
	if err != nil {
		return err
	}
	dref, err := draftlab.ParseAgent(o.drafter)
	if err != nil {
		return fmt.Errorf("--drafter: %w", err)
	}
	bref, err := draftlab.ParseAgent(o.builder)
	if err != nil {
		return fmt.Errorf("--builder: %w", err)
	}
	nd, err := lab.Drafter(dref)
	if err != nil {
		return fmt.Errorf("--drafter: %w", err)
	}
	nb, err := lab.Builder(bref)
	if err != nil {
		return fmt.Errorf("--builder: %w", err)
	}
	coll := setdata.Collation(o.collation)
	if coll == "" {
		coll = setdata.DefaultCollation
	}

	decksDir := filepath.Join(o.out, "decks")
	if err := os.MkdirAll(decksDir, 0o755); err != nil {
		return err
	}
	var index []byte
	var metas []meta
	n := 0
	for i := 0; i < o.pods; i++ {
		podSeed := o.seed + uint64(i)
		packs, err := lab.OpenPod(o.set, coll, podSeed, seats, rounds)
		if err != nil {
			return fmt.Errorf("pod %d (seed %d): %w", i, podSeed, err)
		}
		res, err := lab.Draft(packs, []draftlab.NamedDrafter{nd}, podSeed)
		if err != nil {
			return fmt.Errorf("pod %d (seed %d): %w", i, podSeed, err)
		}
		for seat := 0; seat < seats; seat++ {
			d, err := lab.Build(res.Pools[seat], nb)
			if err != nil {
				return fmt.Errorf("pod %d (seed %d) seat %d: %w", i, podSeed, seat, err)
			}
			id := o.set + "-" + res.DraftIDs[seat]
			m := describe(lab, res.Pools[seat], d)
			metas = append(metas, m)
			f := d.File
			f.Name = id
			f.Format = "limited"
			f.Archetype = m.Archetype
			cards := make([]deck.Entry, len(f.Cards))
			for i, e := range f.Cards {
				cards[i] = e
				c, ok := lab.Card(e.Name)
				if !ok || len(c.Faces) == 0 || c.Faces[0] == nil || slices.Contains(c.Faces[0].Types, "Basic") {
					continue
				}
				cards[i].Name = c.Faces[0].Name
			}
			f.Cards = cards
			rel := filepath.ToSlash(filepath.Join("decks", id+".json"))
			if err := writeJSON(filepath.Join(o.out, rel), f); err != nil {
				return err
			}
			line, err := json.Marshal(indexEntry{
				ID: id, File: rel, Set: o.set, Seed: podSeed, Seat: seat,
				DraftID: res.DraftIDs[seat], Drafter: nd.Ref.String(), Builder: nb.Ref.String(),
				meta: m,
			})
			if err != nil {
				return err
			}
			index = append(append(index, line...), '\n')
			n++
		}
	}
	if err := os.WriteFile(filepath.Join(o.out, "decks.jsonl"), index, 0o644); err != nil {
		return err
	}
	m := lab.NewManifest("deckpool", o.seed, []draftlab.RoleRef{
		{Role: draftlab.RoleDrafter, Ref: nd.Ref},
		{Role: draftlab.RoleBuilder, Ref: nb.Ref},
	}, nil)
	m.Set = o.set
	m.Collation = string(coll)
	m.Mode = "draft"
	m.Seats = seats
	if err := writeJSON(filepath.Join(o.out, "manifest.json"), m); err != nil {
		return err
	}
	if _, err = fmt.Fprintf(stdout, "deckpool: wrote %d decks to %s\n", n, o.out); err != nil {
		return err
	}
	if !o.quiet {
		_, err = fmt.Fprint(stdout, summarise(metas))
	}
	return err
}

// checkOutEmpty refuses a --out that holds anything, before any work.
func checkOutEmpty(dir string) error {
	entries, err := os.ReadDir(dir)
	switch {
	case os.IsNotExist(err):
		return nil
	case err != nil:
		return fmt.Errorf("--out %s: %w", dir, err)
	case len(entries) > 0:
		return fmt.Errorf("--out %s is not empty (deckpool never overwrites; pick a new directory)", dir)
	}
	return nil
}

// writeJSON writes v indented, with a trailing newline.
func writeJSON(path string, v any) error {
	b, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, append(b, '\n'), 0o644)
}
