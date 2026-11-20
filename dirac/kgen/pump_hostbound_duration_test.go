package kgen

import (
	"strings"
	"testing"
)

// TestPumpHostBoundDurationLowers pins the lowering of a Pump's two host
// records beside the keyword-only Permanent grant. Duration$ UntilUntaps is
// "for as long as CARDNAME remains tapped" (registerPumpEffects): a host-bound
// override record (animateOverride's shape, CS WhileHost) whose +N/+M is the
// record's table constants beside its PTDelta, because the kernel keeps the
// host's Obj and version in the record's P/T fields. Duration$ Permanent with
// NumAtt$/NumDef$ (Riding the Dilu Horse, Subversive Acolyte) is the ordinary
// non-expiring PTDelta record. Before this, every one of these refused by
// name and none reached a pool.
func TestPumpHostBoundDurationLowers(t *testing.T) {
	t.Run("UntilUntaps", func(t *testing.T) {
		for _, c := range []struct{ name, delta, kw string }{
			{"Zelyon Sword", "ptdelta=2/0", ""},
			{"Endoskeleton", "ptdelta=0/3", ""},
			{"Ashnod's Battle Gear", "ptdelta=2/-2", ""},
			{"Flowstone Armor", "ptdelta=1/-1", ""},
			{"Tawnos's Weaponry", "ptdelta=1/1", ""},
			{"Spirit Shield", "ptdelta=0/2", ""},
			{"Everglove Courier", "ptdelta=2/2", "kw=Trample"},
			{"Frightshroud Courier", "ptdelta=2/2", "kw=Fear"},
			{"Flamestick Courier", "ptdelta=2/2", "kw=Haste"},
			{"Pearlspear Courier", "ptdelta=2/2", "kw=Vigilance"},
			{"Ghosthelm Courier", "ptdelta=2/2", "kw=Shroud"},
		} {
			t.Run(c.name, func(t *testing.T) {
				cards := lowerNamed(t, []string{c.name})
				if len(cards) != 1 || cards[0].Name != c.name {
					t.Fatalf("lowered %d cards, want %s", len(cards), c.name)
				}
				var reasons []string
				for _, r := range cards[0].Refusals {
					reasons = append(reasons, r.Reason)
				}
				if len(reasons) != 0 {
					t.Fatalf("%s refused: %s", c.name, strings.Join(reasons, "; "))
				}
				var dump strings.Builder
				dumpLIR(&dump, cards[0])
				got := dump.String()
				// the duration is the host-bound record, and its pump the
				// card's literal amounts
				want := []string{
					"Override obj=Target0 CS=WhileHost",
					"override{" + c.delta,
				}
				if c.kw != "" {
					want = append(want, c.kw)
				}
				for _, w := range want {
					if !strings.Contains(got, w) {
						t.Errorf("%s: lowered IR lacks %q:\n%s", c.name, w, got)
					}
				}
			})
		}
	})
	t.Run("Permanent", func(t *testing.T) {
		t.Run("Subversive Acolyte", func(t *testing.T) {
			cards := lowerNamed(t, []string{"Subversive Acolyte"})
			if len(cards) != 1 || cards[0].Name != "Subversive Acolyte" {
				t.Fatalf("lowered %d cards, want Subversive Acolyte", len(cards))
			}
			var reasons []string
			for _, r := range cards[0].Refusals {
				reasons = append(reasons, r.Reason)
			}
			if len(reasons) != 0 {
				t.Fatalf("Subversive Acolyte refused: %s", strings.Join(reasons, "; "))
			}
			var dump strings.Builder
			dumpLIR(&dump, cards[0])
			got := dump.String()
			for _, w := range []string{"CS=Permanent", "ptdelta=1/1", "ptdelta=3/2", "kw=Lifelink", "kw=Trample"} {
				if !strings.Contains(got, w) {
					t.Errorf("lowered IR lacks %q:\n%s", w, got)
				}
			}
		})
		t.Run("Riding the Dilu Horse still refused", func(t *testing.T) {
			// its Duration now reaches the arm (a shape without the keyword
			// lowers, the next subtest), but its KW$ Horsemanship has no
			// grant surface in the kernel: kop.KwMask is full and the
			// granted-Evasion bits cover only Intimidate and Skulk
			// (kw_kwmark.go). A granted horsemanship is a separate ticket.
			cards := lowerNamed(t, []string{"Riding the Dilu Horse"})
			if len(cards) != 1 || cards[0].Name != "Riding the Dilu Horse" {
				t.Fatalf("lowered %d cards, want Riding the Dilu Horse", len(cards))
			}
			var reasons []string
			for _, r := range cards[0].Refusals {
				reasons = append(reasons, r.Reason)
			}
			if len(reasons) == 0 || !strings.Contains(strings.Join(reasons, "; "), `keyword "Horsemanship"`) {
				t.Fatalf("refusals %q, want the horsemanship keyword refusal", strings.Join(reasons, "; "))
			}
		})
		t.Run("Permanent P/T without a keyword lowers", func(t *testing.T) {
			// Riding the Dilu Horse's Duration shape minus KW$: the plain
			// never-expiring PTDelta record this brief lowers
			lc := lowerSynth(t, "Name:Pump Probe\nManaCost:2 G\nTypes:Sorcery\n"+
				"A:SP$ Pump | ValidTgts$ Creature | NumAtt$ +2 | NumDef$ +2 | Duration$ Permanent | SpellDescription$ T gets +2/+2 indefinitely.\n")
			var reasons []string
			for _, r := range lc.Refusals {
				reasons = append(reasons, r.Reason)
			}
			if len(reasons) != 0 {
				t.Fatalf("refused: %s", strings.Join(reasons, "; "))
			}
			var dump strings.Builder
			dumpLIR(&dump, lc)
			got := dump.String()
			for _, w := range []string{"Override obj=Target0 CS=Permanent", "ptdelta=2/2"} {
				if !strings.Contains(got, w) {
					t.Errorf("lowered IR lacks %q:\n%s", w, got)
				}
			}
		})
	})
	t.Run("keeps refusing", func(t *testing.T) {
		for _, c := range []struct{ name, src, want string }{
			{"computed amount", "A:AB$ Pump | Cost$ 2 T | ValidTgts$ Creature | NumAtt$ X | Duration$ UntilUntaps | SpellDescription$ T gets +X/+0 for as long as CARDNAME remains tapped.\nSVar:X:Count$CardsInYourHand", "with a computed P/T pump"},
			{"Perpetual", "A:AB$ Pump | Cost$ 2 T | ValidTgts$ Creature | NumAtt$ +2 | NumDef$ +2 | Duration$ Perpetual | SpellDescription$ T gets +2/+2 indefinitely.", `Pump Duration "Perpetual"`},
			{"no amounts, no keywords", "A:AB$ Pump | Cost$ 2 T | ValidTgts$ Creature | Duration$ UntilUntaps | SpellDescription$ Empty.", `Pump Duration "UntilUntaps" without NumAtt$/NumDef$/KW$`},
		} {
			t.Run(c.name, func(t *testing.T) {
				rs := lowerSynthRefusals(t, c.src)
				var got []string
				for _, r := range rs {
					got = append(got, r.Reason)
				}
				if len(got) == 0 {
					t.Fatalf("no refusals, want %q", c.want)
				}
				if !strings.Contains(strings.Join(got, "; "), c.want) {
					t.Fatalf("refusals %q, want %q", strings.Join(got, "; "), c.want)
				}
			})
		}
	})
}
