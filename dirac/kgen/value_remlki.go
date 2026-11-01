package kgen

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"github.com/adams-shaun/gorge/dirac/kop"
)

// RememberedLKI$CardPower / $CardToughness (vRemLKIPT) reads
// the sum of the characteristic over the creatures a Sacrifice
// RememberSacrificed$ took, from the capture before they left (gorge
// count_ref_property.go, CR 608.2h). The kernel accumulates that sum at the
// sacrifice (resCtx.remLK), so the read is exact only when the remembered
// set IS the sacrificed set: this file refuses every other shape.

var remLKRead = regexp.MustCompile(`RemLKIPT\(n=(\d)\)`)

// remLKFmt prints a filter together with the lowered CmpX bound values it
// carries: an lValue in a filter (a ChooseMove's cmcLEX bound)
// does not appear in lInstr.String, so the remember-read accounting must see
// it here.
func remLKFmt(f *lFilter) string {
	if f == nil {
		return ""
	}
	var b strings.Builder
	for i := range f.Clauses {
		if v := f.Clauses[i].CmpV; v != nil {
			b.WriteString(fmt.Sprint(*v))
			b.WriteByte(' ')
		}
	}
	return b.String()
}

// remLKBits maps a Sacrifice note to its kop.SacSum* bits.
func remLKBits(note string) int {
	switch {
	case strings.HasSuffix(note, ":power"):
		return kop.SacSumPower
	case strings.HasSuffix(note, ":toughness"):
		return kop.SacSumToughness
	case strings.HasSuffix(note, ":mv"):
		return kop.SacSumMV // a trigger's own IfPaid Sacrifice cost
	}
	return 0
}

func remLKSacrifice(in lInstr) bool {
	// a trigger's own IfPaid Sacrifice cost is the same
	// capture (gorge's trigger_cost sacrificed list); it accumulates the one
	// characteristic the reading value names.
	if in.Op == "IfPaid" {
		return in.CS == "Sacrifice" && strings.HasPrefix(in.Note, noteRememberSacrificed)
	}
	return in.Op == "Sacrifice" && strings.HasPrefix(in.Note, noteRememberSacrificed)
}

// refuseRemLKIPT admits a RememberedLKI$ read only in an ability whose
// program has an earlier Sacrifice RememberSacrificed$, no other remember
// writer or ClearRemembered before the read, and no remember writer on the
// card outside that program's Sacrifices (gorge also unions the source's
// persistent list, which must stay empty). It marks the Sacrifices with the
// characteristic they sum.
func (lc *lowerCtx) refuseRemLKIPT() {
	reads := func(l string) bool { return strings.Contains(l, "RemLKIPT(") }
	const shape = "RememberedLKI$ over a set not written by one Sacrifice RememberSacrificed$"
	any := false
	for _, a := range lc.out.Abilities {
		var lines []string
		if a.Trigger != nil {
			t := a.Trigger
			lines = append(lines, remLKFmt(t.F), remLKFmt(t.F2), fmt.Sprint(t.Cond))
		}
		lines = append(lines, fmt.Sprint(a.ActCond))
		for _, t := range a.Targets {
			lines = append(lines, remLKFmt(t.F))
		}
		for _, l := range lines {
			if reads(l) {
				lc.refuseNode(fmt.Sprintf("ab[%d]", a.Index), "%s (a read outside the program)", shape)
				return
			}
		}
		for _, in := range a.Prog {
			any = any || reads(remLKFmt(in.F)+in.String())
		}
	}
	for _, s := range lc.out.Statics {
		any = any || reads(remLKFmt(s.F)) || reads(fmt.Sprint(s.Cond))
	}
	for _, r := range lc.out.Repls {
		for _, in := range r.Prog {
			any = any || reads(in.String())
		}
	}
	if !any && lc.remLKN == 0 {
		return
	}
	// every read lc.value produced must sit in an ability program, where
	// the pass below pairs it with its Sacrifice: a token's TokenPower$, a
	// copy override or any other holder this pass cannot see is refused
	// (Reign of the Pit, Soul Separator)
	inProg := 0
	for _, a := range lc.out.Abilities {
		for _, in := range a.Prog {
			if in.Op == "Discover" && in.C == 1 {
				continue // lc.discover's hand-follow-up repeats the one value its first Discover reads
			}
			inProg += strings.Count(remLKFmt(in.F)+in.String(), "RemLKIPT(")
		}
	}
	if inProg != lc.remLKN {
		lc.refuseNode("card", "%s (a read outside an ability program)", shape)
		return
	}
	// a read in a static or replacement, or another card-level writer
	for _, s := range lc.out.Statics {
		if reads(fmt.Sprint(s.F)) || reads(fmt.Sprint(s.Cond)) {
			lc.refuseNode("card", "%s (a static read)", shape)
			return
		}
	}
	for _, r := range lc.out.Repls {
		for _, in := range r.Prog {
			if reads(in.String()) || kremWrites(in) {
				lc.refuseNode("card", "%s (a replacement)", shape)
				return
			}
		}
	}
	for ai, a := range lc.out.Abilities {
		n, bad := -1, ""
		first := -1 // the first remembering Sacrifice
		cmcAt := -1 // the first Counter RememberCounteredCMC$ (read N 2; lower_counterspell.go)
		sacs := 0
		for _, in := range a.Prog {
			if remLKSacrifice(in) {
				sacs++
			}
		}
		for i, in := range a.Prog {
			s := remLKFmt(in.F) + in.String()
			if m := remLKRead.FindStringSubmatch(s); m != nil {
				v, _ := strconv.Atoi(m[1])
				switch {
				case strings.Count(s, "RemLKIPT(") != 1 || n >= 0 && n != v:
					bad = "two reads, or one beside another operand" // one accumulator, one characteristic
				case v == 2 && (cmcAt < 0 || sacs > 0):
					bad = "no earlier Counter RememberCounteredCMC$, or a Sacrifice RememberSacrificed$ shares the accumulator"
				case v != 2 && first < 0:
					bad = "no earlier Sacrifice RememberSacrificed$"
				case v != 2 && cmcAt >= 0:
					bad = "a Counter RememberCounteredCMC$ shares the accumulator"
				}
				n = v
				continue
			}
			switch {
			case in.Op == "Counter" && in.B.isConst() && in.B.N&kop.CtrRemMV != 0:
				if cmcAt < 0 {
					cmcAt = i
				}
			case remLKSacrifice(in):
				if first < 0 {
					first = i
				}
			case in.Op == "ClearRemembered" && n < 0:
				bad = "ClearRemembered$ before the read"
			case kremWrites(in):
				if n < 0 {
					bad = "another remember writer before the read"
				}
			}
			// a writer that is not a remembering Sacrifice anywhere in a
			// reading ability may feed the persistent list (gorge's union)
			if !remLKSacrifice(in) && kremWrites(in) && bad == "" {
				bad = "another remember writer"
			}
		}
		if n < 0 {
			if first >= 0 || hasWriter(a.Prog) { // a writer in an ability that does not read
				lc.refuseNode(fmt.Sprintf("ab[%d]", a.Index), "%s (a writer in another ability reaches the source's persistent list)", shape)
				return
			}
			continue
		}
		if bad != "" {
			lc.refuseNode(fmt.Sprintf("ab[%d]", a.Index), "%s (%s)", shape, bad)
			return
		}
		suffix := ":power"
		switch n {
		case 1:
			suffix = ":toughness"
		case 3:
			suffix = ":mv" // Sacrificed$CardManaCost
		}
		for i := range lc.out.Abilities[ai].Prog {
			if remLKSacrifice(lc.out.Abilities[ai].Prog[i]) {
				lc.out.Abilities[ai].Prog[i].Note = noteRememberSacrificed + suffix
			}
		}
	}
}

func hasWriter(prog []lInstr) bool {
	for _, in := range prog {
		if kremWrites(in) {
			return true
		}
	}
	return false
}
