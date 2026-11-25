package rules

import (
	"slices"
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/testkit/testutil"
)

// setupOfferedSnap runs one no-step setup scenario and returns its setup
// snapshot: the checkpoint the xproof `compare: [offered]` rows read.
func setupOfferedSnap(t *testing.T, reg *cards.Registry, sc string) OracleSnapshot {
	t.Helper()
	res, err := RunOracleScenarioJSON(reg, []byte(sc))
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Snapshots) != 1 || res.Snapshots[0].Checkpoint != "setup" {
		t.Fatalf("precondition: snapshots = %+v", res.Snapshots)
	}
	return res.Snapshots[0]
}

// TestOracleSnapshotOfferedCastPricesTheUntappedSourceBound pins the setup
// snapshot's `offered` as the CR-legal play set: under CR 601.2g-h a player
// may cast a spell when its cost can be paid by activating mana abilities
// during the announcement, so a {U} Aura with one untapped Island is a legal
// play even though the floating pool alone does not cover it. Forge and XMage
// list it (the side xproof's compare:[offered] trusts); the priority offer's
// gate prices against the floating pool alone by design, so the snapshot
// prices the untapped-source bound instead.
func TestOracleSnapshotOfferedCastPricesTheUntappedSourceBound(t *testing.T) {
	reg := testutil.CorpusRegistry(t)
	const sc = `{"name":"offered-potential-cast","cr":["601.2g","602.2b"],"why":"the setup offered set is the CR-legal one","setup":{"p0":{"hand":["Honest Work"],"battlefield":["Island"]},"p1":{"battlefield":["Wall of Stone"]}},"steps":[]}`
	snap := setupOfferedSnap(t, reg, sc)
	if !slices.ContainsFunc(snap.Players[0].Hand, func(n string) bool { return n == "Honest Work" }) {
		t.Fatalf("precondition: Honest Work in p0's hand: %+v", snap.Players[0].Hand)
	}
	island := false
	for _, p := range snap.Permanents {
		if p.Ref == "p0:Island" {
			island = !p.Tapped
		}
	}
	if !island {
		t.Fatalf("precondition: untapped p0:Island expected, permanents = %+v", snap.Permanents)
	}
	wall := false
	for _, p := range snap.Permanents {
		if p.Ref == "p1:Wall of Stone" && slices.Contains(p.Types, "Creature") && p.Controller == 1 {
			wall = true
		}
	}
	if !wall {
		t.Fatalf("precondition: p1 Wall of Stone creature expected, permanents = %+v", snap.Permanents)
	}
	mana := false
	cast := false
	for _, o := range snap.Offered {
		if o.Source == "p0:Island" && o.Kind == "activate" {
			mana = true
		}
		if o.Source == "p0:Honest Work" && o.Kind == "cast" {
			cast = true
		}
	}
	if !mana {
		t.Fatalf("precondition: p0:Island mana activation expected in offered = %+v", snap.Offered)
	}
	if !cast {
		t.Fatalf("setup offered = %+v, want p0:Honest Work|cast (CR 601.2g: the untapped Island funds the {U} cost during the cast)", snap.Offered)
	}
}

// TestOracleSnapshotOfferedStationManaPricesTheUntappedSourceBound pins the
// same rule for an activation (CR 602.2b): the Station 12+ granted ability of
// Evendo, Waking Haven ({G},{T}: Add {G} for each creature you control) costs
// {G} the floating pool does not hold, which an untapped Forest funds during
// the activation, so the snapshot reports it beside the bare {T}: Add {G}
// ability. Forge and XMage list it.
func TestOracleSnapshotOfferedStationManaPricesTheUntappedSourceBound(t *testing.T) {
	reg := testutil.CorpusRegistry(t)
	const sc = `{"name":"offered-potential-station","cr":["602.2b","702.184"],"why":"the setup offered set is the CR-legal one","setup":{"p0":{"battlefield":["Evendo, Waking Haven","Forest"],"counters":{"Evendo, Waking Haven":{"CHARGE":12}}}},"steps":[]}`
	snap := setupOfferedSnap(t, reg, sc)
	var evendo *OracleSnapPerm
	for i := range snap.Permanents {
		if snap.Permanents[i].Ref == "p0:Evendo, Waking Haven" {
			evendo = &snap.Permanents[i]
		}
	}
	if evendo == nil || evendo.Tapped {
		t.Fatalf("precondition: untapped p0:Evendo, Waking Haven expected, permanents = %+v", snap.Permanents)
	}
	// The bare {T}: Add {G} ability is one of the source's mana labels; the
	// Station 12+ grant ({G},{T}: Add {G} for each creature) is a second,
	// differently-labelled activation of the same source.
	labels := map[string]bool{}
	for _, o := range snap.Offered {
		if o.Source == "p0:Evendo, Waking Haven" && o.Kind == "activate" {
			labels[o.Label] = true
		}
	}
	if len(labels) < 2 {
		t.Fatalf("setup offered = %+v, want the Station 12+ {G},{T} activation beside the bare {T}: Add {G} (CR 602.2b: the untapped Forest funds the {G} fee)", snap.Offered)
	}
	if !labels["Add G"] {
		t.Fatalf("precondition: the bare {T}: Add {G} label expected, labels = %v", labels)
	}
}

// TestOracleSnapshotOfferedGrantedAbilityPricesTheUntappedSourceBound pins
// the non-mana half of the class: the Station 12+ granted ability of Adagia,
// Windswept Bastion ({3}{W},{T}: copy target artifact or enchantment you
// control) is a granted activated ability, priced by the potential-action
// walk's castable gate against the untapped-source bound, with its target
// still required (Memnite, an artifact you control). Forge and XMage list it.
func TestOracleSnapshotOfferedGrantedAbilityPricesTheUntappedSourceBound(t *testing.T) {
	reg := testutil.CorpusRegistry(t)
	const sc = `{"name":"offered-potential-granted","cr":["602.2b","702.184"],"why":"the setup offered set is the CR-legal one","setup":{"p0":{"battlefield":["Adagia, Windswept Bastion","Plains","Plains","Plains","Memnite"],"counters":{"Adagia, Windswept Bastion":{"CHARGE":12}}}},"steps":[]}`
	snap := setupOfferedSnap(t, reg, sc)
	var adagia *OracleSnapPerm
	for i := range snap.Permanents {
		if snap.Permanents[i].Ref == "p0:Adagia, Windswept Bastion" {
			adagia = &snap.Permanents[i]
		}
	}
	if adagia == nil || adagia.Tapped {
		t.Fatalf("precondition: untapped p0:Adagia, Windswept Bastion expected, permanents = %+v", snap.Permanents)
	}
	if !slices.ContainsFunc(snap.Permanents, func(p OracleSnapPerm) bool {
		return p.Ref == "p0:Memnite" && slices.Contains(p.Types, "Artifact")
	}) {
		t.Fatalf("precondition: p0 Memnite artifact expected, permanents = %+v", snap.Permanents)
	}
	copyAbility := false
	for _, o := range snap.Offered {
		if o.Source == "p0:Adagia, Windswept Bastion" && o.Kind == "activate" && strings.Contains(o.Label, "copy") {
			copyAbility = true
		}
	}
	if !copyAbility {
		t.Fatalf("setup offered = %+v, want the Station 12+ copy activation (CR 602.2b: the untapped Plains fund the {3}{W} fee)", snap.Offered)
	}
}
