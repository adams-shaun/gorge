package rules

import (
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/testkit/testutil"
)

// xproofNormalizes is the follower's label normalization, the one the xproof
// `compare: [offered]` applies to both sides of the pair: lowercase, the
// mana-symbol braces and the whitespace gone, the punctuation kept (XMage
// spells the printed ability "addu." with its trailing period).
func xproofNormalizes(s string) string {
	return strings.NewReplacer("{", "", "}", "", " ", "").Replace(strings.ToLower(s))
}

// TestOracleSnapshotOfferedUthrosGrantLabelsTheAbilityText is the regression
// for the xproof setup:offered divergence on Uthros, Titanic Godcore
// (cycles 11251746 s1/s2, 11251823, 11252039, 11260221): the snapshot's
// offered labels for the card's mana activations were composed from the
// resolved ability's cost and production (pay.ManaAbilityLabel), so the
// Station 12+ grant (`Amount$ X`, `X:Count$Valid Artifact.YouCtrl`) with
// zero artifacts on the board rendered "Pay U: Add U" and lost the
// "for each artifact you control" clause XMage keeps, and the printed
// "{T}: Add {U}." rendered "Add U" without the period. XMage spells the
// ability text ("addu.", "adduforeachartifactyoucontrol." normalized); the
// snapshot's offered is the compare surface, so it spells the text too.
func TestOracleSnapshotOfferedUthrosGrantLabelsTheAbilityText(t *testing.T) {
	reg := testutil.CorpusRegistry(t)
	const sc = `{"name":"uthros-station-grant-labels","cr":["702.184"],"why":"the setup offered set spells the ability text","setup":{"p0":{"battlefield":["Island","Uthros, Titanic Godcore"],"counters":{"Uthros, Titanic Godcore":{"CHARGE":12}}}},"steps":[]}`
	snap := setupOfferedSnap(t, reg, sc)
	// Precondition: Uthros is on p0's battlefield, untapped and stationed
	// past 12 -- otherwise the Station 12+ grant below never enters the
	// offered set and the whole test is vacuous.
	uthros := ""
	for _, p := range snap.Permanents {
		if p.Ref == "p0:Uthros, Titanic Godcore" {
			uthros = p.Ref
			if p.Tapped {
				t.Fatalf("precondition: p0:Uthros, Titanic Godcore must be untapped, permanents = %+v", snap.Permanents)
			}
		}
	}
	if uthros == "" {
		t.Fatalf("precondition: p0:Uthros, Titanic Godcore expected on the battlefield, permanents = %+v", snap.Permanents)
	}
	// Precondition: both of Uthros's mana activations are offered -- the
	// printed {T}: Add {U} and the Station 12+ {U},{T} grant. A row dropped
	// by the fix (rather than relabelled) must fail here, not pass vacuously.
	var labels []string
	for _, o := range snap.Offered {
		if o.Source == "p0:Uthros, Titanic Godcore" && o.Kind == "activate" {
			labels = append(labels, o.Label)
		}
	}
	if len(labels) != 2 {
		t.Fatalf("precondition: two p0:Uthros, Titanic Godcore activations expected (printed + Station 12+ grant), offered = %+v", snap.Offered)
	}
	// The Station 12+ grant keeps the printed per-artifact clause, and the
	// printed ability keeps its period: both normalize to the follower's
	// spelling exactly.
	const grant, printed = "adduforeachartifactyoucontrol.", "addu."
	sawGrant, sawPrinted := false, false
	for _, l := range labels {
		switch xproofNormalizes(l) {
		case grant:
			sawGrant = true
		case printed:
			sawPrinted = true
		}
	}
	if !sawGrant {
		t.Fatalf("setup offered for p0:Uthros, Titanic Godcore = %q, want the Station 12+ grant labelled with its ability text (%q)", labels, grant)
	}
	if !sawPrinted {
		t.Fatalf("setup offered for p0:Uthros, Titanic Godcore = %q, want the printed {T}: Add {U} ability labelled with its text (%q)", labels, printed)
	}
}
