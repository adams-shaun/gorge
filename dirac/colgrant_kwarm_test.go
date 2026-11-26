package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/kop"
)

// TestPrepColGrantArmsOnlyProtColGrants pins the colour-word arming
// (prepColGrant, static_colourgrant.go): an OpGrantKeyword arms the
// protection-colour runs (anyProtCol, rbProtCol) only when its keyword mask
// carries a protection-from-a-colour keyword (protColKw = KwProtBlack |
// KwProtRed, both past bit 16). A grant of an ordinary keyword must not:
// any nonzero low half must not arm the ten mechProtW..G runs, the
// protPrinted/protColFrom read paths and rbProtCol for the whole pool.
func TestPrepColGrantArmsOnlyProtColGrants(t *testing.T) {
	protRedHi := int16(kop.KwProtRed >> 16) // the high half's ProtRed bit
	hexInstHi := int16(kop.KwHexproofFromInstants >> 16)
	for _, tc := range []struct {
		name  string
		c     int16 // the grant's low half
		n     int16 // the grant's high half
		armed bool  // whether the prot-colour runs must be armed
	}{
		{"flying only", int16(kop.KwFlying), 0, false},
		{"hexproof-from-instants only", 0, hexInstHi, false},
		{"protection from red", 0, protRedHi, true},
		{"protection from red beside flying", int16(kop.KwFlying), protRedHi, true},
	} {
		tbl := &kop.Table{Cards: []kop.CardDef{{Name: "Test Gainer",
			Abilities: []kop.Ability{{Kind: kop.AbActivated,
				Prog: []kop.Instr{{Op: kop.OpGrantKeyword, C: tc.c,
					B: kop.ValueRef{N: tc.n}}}}}}}}
		g := &genPrep{replBits: make([]uint32, len(tbl.Cards))}
		prepColGrant(tbl, g)
		if got := g.anyProtCol || g.anyColGrant || g.replBits[0]&rbProtCol != 0; got != tc.armed {
			t.Errorf("%s: armed prot-colour reads = %v, want %v (anyProtCol %v anyColGrant %v replBits %#x)",
				tc.name, got, tc.armed, g.anyProtCol, g.anyColGrant, g.replBits[0])
		}
	}
}
