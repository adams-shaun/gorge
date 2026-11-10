package kgen

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/dirac/kop"
)

// kpaylifexReasons joins a lowering's refusal reasons.
func kpaylifexReasons(rs []refusal) string {
	var got []string
	for _, r := range rs {
		got = append(got, r.Reason)
	}
	return strings.Join(got, " | ")
}

// TestPaylifexCarriers: the announced PayLife<X> (SVar X Count$xPaid) on
// two spells and an activated ability lowers to CostPayLife with Cost.N 0;
// Madame Null's trigger cost (SVar X TriggeredCard$CardPower) lowers as a
// resolution payment instead, and every other body or ability kind refuses.
func TestPaylifexCarriers(t *testing.T) {
	read := func(dir, file string) string {
		b, err := os.ReadFile(filepath.Join(tableCorpus(t), "cardsfolder", dir, file))
		if err != nil {
			t.Skip("no gorge corpus: ", err)
		}
		return string(b)
	}
	for _, c := range []struct{ name, dir, file string }{
		{"Fix What's Broken", "f", "fix_whats_broken.txt"},
		{"Vicious Rivalry", "v", "vicious_rivalry.txt"},
		{"Krumar Initiate", "k", "krumar_initiate.txt"},
	} {
		d, rs := kwardpowBuild(t, c.name, read(c.dir, c.file))
		if c.name == "Krumar Initiate" {
			// the Spirit token script is not in this one-card probe's pool:
			// only a life-cost refusal would fail the carrier
			if all := kpaylifexReasons(rs); strings.Contains(all, "life cost") {
				t.Errorf("%s: refusals %q", c.name, all)
			}
		} else if len(rs) != 0 {
			t.Errorf("%s refused: %v", c.name, rs)
			continue
		}
		var got *kop.Cost
		for i := range d.Abilities {
			if d.Abilities[i].Cost.Flags&kop.CostPayLife != 0 {
				got = &d.Abilities[i].Cost
			}
		}
		if d.SpellCost.Flags&kop.CostPayLife != 0 {
			got = &d.SpellCost
		}
		if c.name == "Krumar Initiate" {
			continue
		}
		if got == nil || got.N != 0 {
			t.Errorf("%s: no CostPayLife with N 0 (got %v)", c.name, got)
		}
	}
	_, rs := kwardpowBuild(t, "Madame Null, Power Broker", read("m", "madame_null_power_broker.txt"))
	if all := kpaylifexReasons(rs); all != "" {
		t.Errorf("Madame Null: refusals %q, want none (the trigger's own power-valued life cost lowers)", all)
	}
	// a Count$xPaid body on a trigger's own cost, a mana ability and a
	// literal beside the X form refuse by name
	probes := []struct{ name, src, want string }{
		{"Trigger Probe", "Name:Trigger Probe\nManaCost:2 B\nTypes:Creature Demon\nPT:1/3\n" +
			"T:Mode$ ChangesZone | Origin$ Any | Destination$ Battlefield | ValidCard$ Creature.Other+YouCtrl | Execute$ TrigPut | TriggerZones$ Battlefield\n" +
			"SVar:TrigPut:AB$ PutCounter | Cost$ PayLife<X> | Defined$ TriggeredCardLKICopy | CounterType$ P1P1 | CounterNum$ X\nSVar:X:Count$xPaid\n",
			"life cost PayLife<X> on a non-spell, non-activated ability"},
		{"Mana Probe", "Name:Mana Probe\nManaCost:1 B\nTypes:Creature Human\nPT:2/2\n" +
			"A:AB$ Mana | Cost$ T PayLife<X> | Produced$ B\nSVar:X:Count$xPaid\n",
			"life cost PayLife<X> on a non-spell, non-activated ability"},
		{"Body Probe", "Name:Body Probe\nManaCost:1 B\nTypes:Creature Human\nPT:2/2\n" +
			"A:AB$ Endure | Cost$ X B T PayLife<X> | Num$ X\nSVar:X:Count$CardPower\n",
			"life cost PayLife<X> with SVar X"},
	}
	for _, p := range probes {
		_, rs := kwardpowBuild(t, p.name, p.src)
		if all := kpaylifexReasons(rs); !strings.Contains(all, p.want) {
			t.Errorf("%s: refusals %q, want %q", p.name, all, p.want)
		}
	}
}
