package effects

import (
	"strings"

	"github.com/adams-shaun/gorge/engine/state"
)

// costHeadCode is the evalCountBodyCostCodes code of a head. Count$DamageThisTurn
// shares its word with the PlayerCount property of the same name, so it takes
// that table's code rather than a key of its own.
func costHeadCode(head string) evalCountBodyCostCode {
	if cov3PlayerPropCodes.Code(head) == cov3PlayerPropDamageThisTurn {
		return evalCountBodyCostDamageThisTurn
	}
	return evalCountBodyCostCodes.Code(head)
}

func evalDamageHistory(h Host, c *Ctx, g *state.Game, code evalCountBodyCostCode, arg string) (int32, bool, bool) {
	if !isDamageHistoryCode(code) {
		return 0, false, false
	}
	if code == evalCountBodyCostMaxCombatDamageThisTurn {
		return maxPlayerCombatDamage(g), true, true
	}
	fields := strings.Fields(arg)
	if len(fields) == 0 {
		return 0, false, true
	}
	sourceSpec, recipientSpec := fields[0], "Any"
	if len(fields) > 1 {
		recipientSpec = fields[1]
	} else if code == evalCountBodyCostTotalDamageThisTurn {
		// TotalDamageThisTurn names only the damaging sources: the recipient
		// is the host card itself (Blazing Effigy's "dealt to CARDNAME").
		recipientSpec = "Card.Self"
	}
	var total int32
	for i := range g.Objs {
		source := &g.Objs[i]
		if source.ID == 0 {
			continue
		}
		for _, hit := range source.DamageDealtThisTurn {
			if !damageSourceMatches(g, sourceSpec, source, hit, c) ||
				code == evalCountBodyCostNonCombatDamageThisTurn && hit.Combat ||
				code == evalCountBodyCostNumCombatDamageThisTurn && !hit.Combat {
				continue
			}
			if damageRecipientMatches(g, recipientSpec, hit, c) {
				if code == evalCountBodyCostNumDamageThisTurn || code == evalCountBodyCostNumCombatDamageThisTurn {
					total++ // A source qualifies once, regardless of hits or amount.
					break
				}
				total += hit.Amount
			}
		}
	}
	return total, true, true
}

// Match each hit using its source's recorded characteristics when damage landed.
// Older records without a zone/type snapshot retain the live filter semantics.
func damageSourceMatches(g *state.Game, spec string, source *state.Object, hit state.DamageDealtRecord, c *Ctx) bool {
	snapshot := *source
	snapshot.Controller = hit.SourceControl
	if hit.HasSourceZone {
		snapshot.Zone = hit.SourceZone
	}
	sc := c.SpecContext(c.Controller)
	if hit.SourceTypes != nil {
		// The hit's complete effective type list overrides any current layer-4
		// entry, including one that appeared after the damage was dealt.
		types := make([]ObjectTypes, 0, len(sc.Layers.DerivedTypes)+1)
		for _, dt := range sc.Layers.DerivedTypes {
			if dt.ID != source.ID {
				types = append(types, dt)
			}
		}
		sc.Layers.DerivedTypes = append(types, ObjectTypes{ID: source.ID, Types: hit.SourceTypes})
	}
	if hit.HasSourceColors {
		// The source's colour is read as it was when the damage was dealt
		// (a red Ojer that later returns as its colourless Temple face still
		// counts its earlier hits as red).
		colors := make([]ObjectColors, 0, len(sc.Layers.DerivedColors)+1)
		for _, dc := range sc.Layers.DerivedColors {
			if dc.ID != source.ID {
				colors = append(colors, dc)
			}
		}
		sc.Layers.DerivedColors = append(colors, ObjectColors{ID: source.ID, Mask: ColorMaskFromLetters(hit.SourceColors)})
	}
	if snapshot.Zone == state.ZBattlefield {
		return matchesObjectPtr(g, spec, &snapshot, &sc)
	}
	if source.IsCopy && snapshot.Zone != state.ZStack {
		return false
	}
	return compiledMatchZone(compiledSpecFor(spec), g, &snapshot, &sc, snapshot.Zone)
}

// MaxCombatDamageThisTurn asks how much combat damage any ONE player took,
// across all sources. Damage to permanents is not damage to their controller.
func maxPlayerCombatDamage(g *state.Game) int32 {
	perPlayer := make([]int32, len(g.Players))
	for i := range g.Objs {
		for _, hit := range g.Objs[i].DamageDealtThisTurn {
			if p, ok := hit.Recipient.PlayerRef(); hit.Combat && ok && p >= 0 && int(p) < len(perPlayer) {
				perPlayer[p] += hit.Amount
			}
		}
	}
	var max int32
	for _, n := range perPlayer {
		if n > max {
			max = n
		}
	}
	return max
}

func damageRecipientMatches(g *state.Game, spec string, hit state.DamageDealtRecord, c *Ctx) bool {
	if player, ok := hit.Recipient.PlayerRef(); ok {
		return MatchesPlayerSpec(g, spec, player, c.Controller)
	}
	o := g.Obj(hit.Recipient)
	if o == nil {
		return false
	}
	// Damage filters describe the recipient when damage was dealt. In
	// particular, a permanent remains a permanent for this count after lethal
	// damage moves it to another zone.
	snapshot := *o
	snapshot.Zone = hit.RecipientZone
	snapshot.Controller = hit.RecipientControl
	sc := c.SpecContext(c.Controller)
	if hit.RecipientTypes != nil {
		types := append([]ObjectTypes(nil), sc.Layers.DerivedTypes...)
		found := false
		for i := range types {
			if types[i].ID == snapshot.ID {
				types[i].Types = hit.RecipientTypes
				types[i].AllCreatureTypes = false
				found = true
				break
			}
		}
		if !found {
			types = append(types, ObjectTypes{ID: snapshot.ID, Types: hit.RecipientTypes})
		}
		sc.Layers.DerivedTypes = types
	}
	return matchesObjectPtr(g, spec, &snapshot, &sc)
}

// evalBloodthirstAmount is the bare Count$BloodthirstAmount head (Notorious
// Throng's "X is the damage dealt to your opponents this turn" token count):
// the total damage that LANDED this turn on the resolving controller's
// opponents, summed over the per-object DamageDealtThisTurn records the
// DamageProvenance fold fills -- combat and noncombat, every source, the
// same ledger maxPlayerCombatDamage folds. The read is
// damageToOppsThisTurn, the shared helper the PlayerCountPropertyYou$
// DamageToOppsThisTurn arm and the TriggeredPlayer$-ref arm already answer
// "damage dealt to your opponents" with, so the three spellings cannot
// drift; it sums the Host's per-player damage fold over the living seats
// that are not the resolving controller. A bare head; an argument is a
// shape this build does not model, so fail the head's own verdict.
func evalBloodthirstAmount(h Host, c *Ctx, g *state.Game, arg string) (int32, bool, bool) {
	if strings.TrimSpace(arg) != "" {
		return 0, false, true
	}
	if c == nil {
		return 0, false, true
	}
	return damageToOppsThisTurn(h, g, c.Controller), true, true
}

// evalSingleMaxDamageThisTurn is the bare Count$SingleMaxDamageThisTurn head
// (Impact Resonance's "X is the greatest amount of damage dealt by a source
// to a permanent or player this turn"): the largest per-(source, recipient)
// total over each source's DamageDealtThisTurn records. A trampler that dealt
// 3 to a blocker and 2 to a player reads 3, not 5. No controller filter
// (Impact Resonance reads ANY source, whoever controlled it when the hit
// landed). The reduction is a max, so the walk order reaches nothing. A bare
// head; an argument is a shape this build does not model, so fail the head's
// own verdict.
func evalSingleMaxDamageThisTurn(g *state.Game, arg string) (int32, bool, bool) {
	if strings.TrimSpace(arg) != "" {
		return 0, false, true
	}
	var best int32
	for i := range g.Objs {
		hits := g.Objs[i].DamageDealtThisTurn
		for j := range hits {
			var sum int32
			for k := range hits {
				if hits[k].Recipient == hits[j].Recipient {
					sum += hits[k].Amount
				}
			}
			best = max(best, sum)
		}
	}
	return best, true, true
}
