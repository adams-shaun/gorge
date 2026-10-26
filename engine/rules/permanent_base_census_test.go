package rules

// The class ratchet for CR 110.4a: a bare Permanent filter base is the
// battlefield test only where its candidates sit on the battlefield; a reader
// whose candidates are all off it (a hand, graveyard, library) must read it as
// the permanent-card type test. See TestPermanentBaseOffBattlefieldReadersNormalise.

import (
	"regexp"
	"sort"
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/testkit/testutil"
)

var pbBare = regexp.MustCompile(`(^|,)\s*!?Permanent($|[.+,])`)

func pbOffZones(v string) bool {
	if strings.TrimSpace(v) == "" {
		return false
	}
	for _, z := range strings.Split(v, ",") {
		switch strings.ToLower(strings.TrimSpace(z)) {
		case "hand", "graveyard", "library", "exile", "sideboard", "command", "stack":
		default:
			return false
		}
	}
	return true
}

// pbSAClasses is every (API, parameter) of one ability whose value names a
// bare Permanent base while the ability reads only non-battlefield zones.
func pbSAClasses(sa *cards.SA, out map[string]string) {
	keys := make([]string, 0, len(sa.Params))
	for k := range sa.Params {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	zoneKey := map[string]string{"ChooseCard": "ChoiceZone", "Play": "ValidZone", "AnimateAll": "Zone"}[sa.API]
	for _, k := range keys {
		v := sa.Params[k]
		if !pbBare.MatchString(v) || strings.HasPrefix(k, "Condition") || strings.Contains(k, "Present") {
			continue
		}
		off := false
		switch {
		case sa.API == "Discard" && k == "DiscardValid":
			off = true
		case zoneKey != "" && (k == "Choices" || k == "Valid" || k == "ValidCards"):
			off = pbOffZones(sa.Params[zoneKey])
		case k == "ChangeType" || k == "ChangeValid":
			off = pbOffZones(sa.Params["Origin"])
		case k == "ValidTgts":
			off = pbOffZones(sa.Params["TgtZone"])
		}
		if off {
			out["api:"+sa.API+"."+k] = sa.Params[k]
		}
	}
}

func pbTriggerClasses(t *cards.Trigger, out map[string]string) {
	if t.Mode != "ChangesZone" && t.Mode != "ChangesZoneAll" {
		return
	}
	dest := t.Params["Destination"]
	if !pbOffZones(dest) {
		return
	}
	for _, z := range strings.Split(t.Params["Origin"], ",") {
		if strings.EqualFold(strings.TrimSpace(z), "battlefield") {
			return
		}
	}
	for _, k := range []string{"ValidCard", "ValidCards"} {
		if v := t.Params[k]; pbBare.MatchString(v) {
			out["trig:"+t.Mode+"."+k] = v
		}
	}
}

// permanentBaseCarriers is the corpus' off-battlefield bare-Permanent readers,
// class -> sorted card names.
func permanentBaseCarriers(t *testing.T) map[string][]string {
	reg := testutil.CorpusRegistry(t)
	byClass := map[string]map[string]bool{}
	for _, c := range reg.AllCards() {
		for _, f := range c.Faces {
			found := map[string]string{}
			var walk func(sa *cards.SA)
			seen := map[*cards.SA]bool{}
			walk = func(sa *cards.SA) {
				for ; sa != nil && !seen[sa]; sa = sa.Sub {
					seen[sa] = true
					pbSAClasses(sa, found)
				}
			}
			for _, sa := range f.Abilities {
				walk(sa)
			}
			f.EachSVarAbility(walk)
			for i := range f.Triggers {
				pbTriggerClasses(&f.Triggers[i], found)
				walk(f.Triggers[i].Effect)
			}
			f.EachRawEffectChild(func(ch cards.EffectChild) {
				if ch.Trigger != nil {
					pbTriggerClasses(ch.Trigger, found)
				}
			})
			for cl := range found {
				if byClass[cl] == nil {
					byClass[cl] = map[string]bool{}
				}
				byClass[cl][f.Name] = true
			}
		}
	}
	out := map[string][]string{}
	for cl, names := range byClass {
		for n := range names {
			out[cl] = append(out[cl], n)
		}
		sort.Strings(out[cl])
	}
	return out
}

// permanentBaseWildcardParams are the parameters whose off-battlefield bare
// Permanent value is normalised for every API by one shared reader:
// permanentCardSpec (ChangeType$/ChangeValid$ of the search, dig and hand
// paths) and targetSpecForZone (ValidTgts$).
var permanentBaseWildcardParams = map[string]string{
	"ChangeType":  "permanentCardSpec",
	"ChangeValid": "permanentCardSpec",
	"ValidTgts":   "targetSpecForZone",
}

// permanentBaseProbes are the classes whose reader matches each candidate in
// its own zone (matchesInOwnZone / the trigger's SpecOpts.Zone). Each probe is
// an oracle scenario over real carrier cards holding a Grizzly Bears (a
// permanent card, must match) and a Lightning Bolt (must not) in the reader's
// zone; reject, when set, is the same scenario naming the Bolt, which must be
// refused as not offered.
var permanentBaseProbes = map[string]struct{ run, reject string }{
	"api:Discard.DiscardValid":       {run: solveScenario("Grizzly Bears"), reject: solveScenario("Lightning Bolt")},
	"api:Play.Valid":                 {run: kellanScenario("Grizzly Bears"), reject: kellanScenario("Lightning Bolt")},
	"api:ChooseCard.Choices":         {run: collectorScenario},
	"api:AnimateAll.ValidCards":      {run: kamiScenario},
	"trig:ChangesZone.ValidCard":     {run: grolnokScenario},
	"trig:ChangesZoneAll.ValidCards": {run: moonshadowScenario},
}

// TestPermanentBaseOffBattlefieldReadersNormalise walks the corpus for every
// (API or trigger Mode, parameter) that pairs a bare Permanent base with a
// reader seeing only non-battlefield zones. Each class must be a probed
// zone-aware reader or a wildcard-normalised parameter; a new class fails, so
// the next off-battlefield reader cannot be missed. Each probed class then
// runs its behavioural probe.
func TestPermanentBaseOffBattlefieldReadersNormalise(t *testing.T) {
	t.Parallel()
	classes := permanentBaseCarriers(t)
	var keys []string
	for k := range classes {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		param := k[strings.LastIndex(k, ".")+1:]
		if _, ok := permanentBaseProbes[k]; ok {
			continue
		}
		if _, ok := permanentBaseWildcardParams[param]; ok {
			continue
		}
		t.Errorf("class %s (%s) reads a bare Permanent base off the battlefield with no zone-aware reader or probe; "+
			"match it with matchesInOwnZone/permanentCardSpec and add a probe", k, strings.Join(classes[k], ", "))
	}
	for k := range permanentBaseProbes {
		if _, ok := classes[k]; !ok {
			t.Errorf("probe %s names a class the corpus no longer carries", k)
		}
	}
	probeKeys := make([]string, 0, len(permanentBaseProbes))
	for k := range permanentBaseProbes {
		probeKeys = append(probeKeys, k)
	}
	sort.Strings(probeKeys)
	for _, k := range probeKeys {
		p := permanentBaseProbes[k]
		t.Run(k, func(t *testing.T) {
			runPermanentBaseScenario(t, p.run)
			if p.reject == "" {
				return
			}
			fails, _, _ := runOracleScenario(testutil.CorpusRegistry(t), mustPermanentBaseScenario(t, p.reject))
			if !strings.Contains(strings.Join(fails, "\n"), "not offered") {
				t.Fatalf("a Lightning Bolt was offered to a Permanent-base reader: %v", fails)
			}
		})
	}
}
