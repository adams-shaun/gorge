package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/events"
)

// CR 114.2: an emblem is controlled by the player who gets it. A
// "whenever a source you control deals noncombat damage to an opponent"
// trigger (Thor, Guardian of Midgard's line) on the emblem's CREATOR's side
// must not fire when the emblem deals damage to its owner: that source is the
// owner's, not the creator's (Chandra, Awakened Inferno's +2 emblem).

const emblemThorSrc = "Name:Test Thunder Watcher\nManaCost:2 R\nTypes:Creature Giant\nPT:5/5\n" +
	"T:Mode$ DamageDone | TriggerZones$ Battlefield | ValidTarget$ Opponent | ValidSource$ Card.YouCtrl,Emblem.YouCtrl | CombatDamage$ False | OptionalDecider$ You | Execute$ WatchGain | TriggerDescription$ x\n" +
	"SVar:WatchGain:DB$ GainLife | LifeAmount$ 1 | Defined$ You\n" +
	"Oracle:x\n"

// emblemSelfWatchSrc is the control: the emblem OWNER's "whenever a source you
// control deals noncombat damage to you" does see the emblem as theirs, so the
// trigger channel is live for this damage.
const emblemSelfWatchSrc = "Name:Test Self Watcher\nManaCost:2 R\nTypes:Creature Giant\nPT:1/1\n" +
	"T:Mode$ DamageDone | TriggerZones$ Battlefield | ValidTarget$ You | ValidSource$ Card.YouCtrl,Emblem.YouCtrl | CombatDamage$ False | Execute$ WatchGain | TriggerDescription$ x\n" +
	"SVar:WatchGain:DB$ GainLife | LifeAmount$ 1 | Defined$ You\n" +
	"Oracle:x\n"

func TestEmblemDamageDoesNotTriggerTheCreatorsSourceYouControl(t *testing.T) {
	t.Parallel()
	var watcher, control = new(int), new(int)
	e, maker, mark := emblemUpkeep(t, func(e *Engine) {
		*watcher = int(onBoard(t, e, 1, emblemThorSrc))
		*control = int(onBoard(t, e, 0, emblemSelfWatchSrc))
	})
	dealt := false
	for _, ev := range e.L.Events[mark:] {
		if ev.Kind == events.Damage && ev.Player == 0 {
			dealt = true
		}
	}
	if !dealt {
		t.Fatal("precondition: the emblem dealt no damage to its owner")
	}
	if e.G.Obj(maker).Controller != 1 {
		t.Fatal("precondition: the emblem's creator is not seat 1")
	}
	fired := map[int]bool{}
	for _, pt := range e.pendingTriggers {
		fired[int(pt.Source)] = true
	}
	for _, id := range e.G.Stack {
		if o := e.G.Obj(id); o != nil {
			fired[int(o.Source)] = true
		}
	}
	if !fired[*control] {
		t.Fatal("precondition: the emblem owner's source-you-control trigger did not fire on the emblem's damage")
	}
	if fired[*watcher] {
		t.Errorf("the creator's source-you-control damage trigger fired on the emblem owner's emblem damage")
	}
}
