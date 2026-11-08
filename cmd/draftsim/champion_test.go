package main

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/draft/limited/pickbot"
	"github.com/adams-shaun/gorge/engine/cards"
)

func TestDraftsimChampionPersona(t *testing.T) {
	dir := corpusReady(t)
	o := options{
		set: "FDN", seed: 1, events: 1, players: "bot,bot,bot,bot",
		drafter: "champion", builder: "champion", structure: "none", dir: dir,
	}
	res, err := run(o, &strings.Builder{})
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	for _, er := range res.events {
		if len(er.games) != 0 {
			t.Fatalf("played %d games under structure none", len(er.games))
		}
	}
	d, b, err := personaPair("champion", "champion", func(string) *cards.Card { return nil })
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := d.(*pickbot.Knobbed); !ok {
		t.Errorf("drafter is %T", d)
	}
	if _, ok := b.(*pickbot.Best23); !ok {
		t.Errorf("builder is %T", b)
	}
}

func TestDraftsimChampionNeedsFDN(t *testing.T) {
	o := options{
		set: "BLB", seed: 1, events: 1, players: "bot,bot,bot,bot",
		drafter: "champion", builder: "champion", structure: "none",
		dir: filepath.Join(t.TempDir(), "missing"),
	}
	_, err := run(o, &strings.Builder{})
	if err == nil || !strings.Contains(err.Error(), "champion") || !strings.Contains(err.Error(), "FDN") {
		t.Fatalf("err = %v", err)
	}
}
