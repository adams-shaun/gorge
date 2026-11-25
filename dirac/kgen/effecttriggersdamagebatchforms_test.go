package kgen

import (
	"strings"
	"testing"
)

// TestEffectTriggersDamageBatchForms: an end-of-turn Effect's
// Mode$ DamageDoneOnce line lowers as the per-referent DamageDone emblem line
// (Tf2PerReferent) whose fireEmblemsDmg hook latches one instance per damaged
// player per damage batch. The gate's own refusals have no corpus card
// reaching them (grep the corpus's Mode$ DamageDoneOnce Effect triggers:
// Aphelia and The Raven's Warning are the two player-referent shapes, The
// Raven's Warning stays refused by its wish ability), so synthetic faces pin
// each below.
func TestEffectTriggersDamageBatchForms(t *testing.T) {
	checkForms(t, []formCase{
		{card: "Aphelia, Viper Whisperer", form: "EOT Effect DamageDoneOnce, per-referent batch emblem line", lir: "CreateEmblem"},
		{card: "Green Dragon", form: "EOT Effect DamageDoneOnce to creatures stays refused (body reads the damaged object the emblem line does not carry)", refused: "Effect DamageDoneOnce body reads TriggeredTargetLKICopy"},
		{card: "Jace, Cunning Castaway", form: "Duration$ EndOfTurn Effect stays refused", refused: "Effect triggers without a permanent duration"},
	})
	const base = "Name:Probe\nManaCost:2 B\nTypes:Creature Gorgon\nPT:1/1\n"
	for _, tc := range []struct{ name, body, want string }{
		{
			name: "UntilYourNextTurn still refuses at the duration gate",
			body: "A:AB$ Effect | Cost$ 4 B | Duration$ UntilYourNextTurn | Triggers$ TrigDamage | SpellDescription$ test\n" +
				"SVar:TrigDamage:Mode$ DamageDoneOnce | CombatDamage$ True | ValidSource$ Creature.YouCtrl | ValidTarget$ Player | Execute$ TrigLife | TriggerDescription$ test\n" +
				"SVar:TrigLife:DB$ LoseLife | Defined$ TriggeredTarget | LifeAmount$ 1\n",
			want: "Effect triggers without a permanent duration",
		},
		{
			name: "DamageAmount$ on the batch total refuses by name",
			body: "A:AB$ Effect | Cost$ 4 B | Triggers$ TrigDamage | SpellDescription$ test\n" +
				"SVar:TrigDamage:Mode$ DamageDoneOnce | CombatDamage$ True | ValidSource$ Creature.YouCtrl | ValidTarget$ Player | DamageAmount$ GE1 | Execute$ TrigLife | TriggerDescription$ test\n" +
				"SVar:TrigLife:DB$ LoseLife | Defined$ TriggeredTarget | LifeAmount$ 1\n",
			want: "Effect DamageDoneOnce trigger with DamageAmount",
		},
		{
			name: "TriggeredCardController read refuses (the per-event line's trigger player is the damaged seat, not the source's controller)",
			body: "A:AB$ Effect | Cost$ 4 B | Triggers$ TrigDamage | SpellDescription$ test\n" +
				"SVar:TrigDamage:Mode$ DamageDoneOnce | CombatDamage$ True | ValidSource$ Creature.YouCtrl | ValidTarget$ Player | Execute$ TrigLife | TriggerDescription$ test\n" +
				"SVar:TrigLife:DB$ LoseLife | Defined$ TriggeredCardController | LifeAmount$ 1\n",
			want: "Effect DamageDoneOnce body reads TriggeredCardController",
		},
	} {
		if got := keclReason(keclSynth(t, "Probe", base+tc.body)); !strings.Contains(got, tc.want) {
			t.Errorf("%s: refusals %q, want one containing %q", tc.name, got, tc.want)
		}
	}
}
