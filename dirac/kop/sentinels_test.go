package kop

import (
	"strings"
	"testing"
)

// kopSentinelFamilies is every declared prefix whose members share one operand
// field: two members holding one value make two meanings ride the same
// instruction field, and nothing else catches it (2026-11-04: PayBSubChain and
// PayBSubExileHand both took 8 on PayExileGrave's B.N until the ordinals rows
// landed and a hand exile and a chained graveyard exile became one
// instruction). Each entry names the field the family rides.
var kopSentinelFamilies = []struct {
	prefix string
	field  string
}{
	// Instr.B (B.N) of the cost op each member's comment names (PaySac,
	// PayManaX, PayExileGrave, PaySacSelf): dirac/kop/costs.go.
	{"PayBSub", "Instr.B of a cost op"},
	// Instr.B (B.N) of an OpUnlessPaid PayDiscard unless-cost: dirac/kop/costs.go.
	{"PayB", "Instr.B of a PayDiscard unless-cost"},
	// OpMayAsk's C guard: dirac/kop/operands.go.
	{"MayGuard", "OpMayAsk C"},
	// OpOverride/OpMayPlay's Duration: dirac/kop/operands.go.
	{"Dur", "OpOverride/OpMayPlay duration"},
	// OpToLibrary/OpToLibraryChoice's C placement: dirac/kop/operands.go.
	{"Lib", "library placement C"},
	// OpDelayedProgram's A timing: dirac/kop/operands.go.
	{"Delay", "OpDelayedProgram A"},
}

// TestKopSentinelFamiliesDistinct holds every member of one sentinel family to
// a distinct value. The constants are untyped ints declared one by one, so the
// compiler cannot catch a reused value; this reads the package's constants the
// way TestKopOrdinalsUnchanged does and fails naming both members of a family
// that share one.
func TestKopSentinelFamiliesDistinct(t *testing.T) {
	rows := constantRows(t)
	members := map[string][]string{}
	for _, row := range rows {
		parts := strings.Split(row, "\t")
		if len(parts) != 3 || !strings.HasPrefix(parts[1], "untyped ") {
			continue
		}
		fam := ""
		for _, f := range kopSentinelFamilies {
			if strings.HasPrefix(parts[0], f.prefix) && len(f.prefix) > len(fam) {
				fam = f.prefix
			}
		}
		if fam != "" {
			members[fam] = append(members[fam], parts[0]+"\t"+parts[2])
		}
	}
	for _, f := range kopSentinelFamilies {
		got := members[f.prefix]
		if len(got) == 0 {
			t.Fatalf("family %s matched no untyped constant; the prefix list is stale", f.prefix)
		}
		seen := map[string]string{}
		for _, m := range got {
			name, value, _ := strings.Cut(m, "\t")
			if prev, ok := seen[value]; ok {
				t.Errorf("%s and %s in family %s both hold %s on %s", prev, name, f.prefix, value, f.field)
			} else {
				seen[value] = name
			}
		}
	}
}
