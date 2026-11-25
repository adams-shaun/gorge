package rules

import (
	"reflect"
	"testing"

	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/state"
	"github.com/adams-shaun/gorge/testkit/testutil"
)

func TestOracleSnapshotNamedManaOffers(t *testing.T) {
	reg := testutil.CorpusRegistry(t)
	for _, tc := range []struct {
		name              string
		battlefield, want []string
	}{
		{"one ability", []string{"Mountain"}, []string{"Add R"}},
		{"two abilities", []string{"Blazemire Verge", "Swamp"}, []string{"Add B", "Add R"}},
		{"gated out red ability", []string{"Blazemire Verge"}, []string{"Add B"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			fails, _, r := runOracleScenario(reg, oracleScenario{Setup: map[string]oracleSeat{"p0": {Battlefield: tc.battlefield}}})
			if len(fails) > 0 {
				t.Fatal(fails)
			}
			ref := "p0:" + tc.battlefield[0]
			id, err := r.resolve(ref)
			if err != nil {
				t.Fatal(err)
			}
			if o := r.e.G.Obj(id); o == nil || o.Zone != state.ZBattlefield {
				t.Fatalf("precondition: %s must be on battlefield: %+v", ref, o)
			}
			labels := r.manaAbilityLabels(0, id)
			if !reflect.DeepEqual(labels, tc.want) {
				t.Fatalf("precondition: authoritative available abilities=%v, want %v", labels, tc.want)
			}
			d := r.e.Pending()
			if d == nil || d.Kind != decision.KPriority || d.Player != 0 {
				t.Fatalf("precondition: priority for p0 required: %+v", d)
			}
			generic := 0
			for _, o := range d.Options {
				if o.Obj == id && o.Kind == "activate" {
					generic++
				}
			}
			if generic != 1 {
				t.Fatalf("precondition: one generic mana action required, got %d", generic)
			}
			var got []string
			for _, o := range r.snapshot("setup").Offered {
				if o.Source == ref && o.Kind == "activate" {
					got = append(got, o.Label)
				}
			}
			if !reflect.DeepEqual(got, tc.want) {
				t.Fatalf("snapshot named abilities=%v, want %v (not the generic display action)", got, tc.want)
			}
		})
	}
}

func TestOracleSnapshotOffersClosedVocabulary(t *testing.T) {
	reg := testutil.CorpusRegistry(t)
	fails, _, r := runOracleScenario(reg, oracleScenario{Setup: map[string]oracleSeat{"p0": {Battlefield: []string{"Mountain"}}}})
	if len(fails) > 0 {
		t.Fatal(fails)
	}
	id, err := r.resolve("p0:Mountain")
	if err != nil || r.e.G.Obj(id).Zone != state.ZBattlefield {
		t.Fatalf("precondition: battlefield source: %v", err)
	}
	// This tests only the wire mapping, not the legality of these abilities:
	// the input stands in for the priority walk's already-filtered options.
	// The snapshot's offered set also carries the potential-action walk's
	// plays and mana labels (oracle_snapshot_offers.go), so the assertion
	// keys on the input option's own label: a non-wire kind contributes no
	// row with it, a wire kind exactly one, with its kind mapped and its
	// source and label retained.
	for _, tc := range []struct{ kind, want string }{
		{"cast", "cast"}, {"play_land", "play"}, {"play", "play"},
		{"ability", "activate"}, {"granted", "activate"},
		{"station", "activate"}, {"unlock", "activate"}, {"turn_face_up", "activate"}, {"specialize", "activate"},
		{"pass", ""}, {"concede", ""},
	} {
		t.Run(tc.kind, func(t *testing.T) {
			d := &decision.Decision{Player: 0, Kind: decision.KPriority, Options: []decision.Option{{Obj: id, Kind: tc.kind, Label: "named action"}}}
			got := oracleSnapshotOffers(r, d)
			var wire []OracleSnapOffered
			for _, row := range got {
				if row.Label == "named action" {
					wire = append(wire, row)
				}
			}
			if tc.want == "" {
				if len(wire) != 0 {
					t.Fatalf("non-wire action exported: %+v", wire)
				}
			} else if len(wire) != 1 || wire[0].Kind != tc.want || wire[0].Source != "p0:Mountain" {
				t.Fatalf("wire action=%+v, want kind %s with source/label retained", got, tc.want)
			}
		})
	}
}
