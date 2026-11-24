// Package diff compares cardc's definition of a card with Forge's, item by item
// and field by field, after a fixed set of value canonicalisations.
package diff

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"sort"
	"strconv"
	"strings"

	"github.com/adams-shaun/gorge/engine/cards"
)

// Row is one difference between cardc's card and Forge's.
type Row struct {
	Card   string // the card name
	Face   int    // face index
	Item   string // the item's shape key and its index among equal keys: "T:ChangesZone:Draw#0"
	Field  string // "-" for an unpaired item or a face-count row; "[k]<Key>" on chain node k; "<Key>" on the line
	Family string // see families; "shape" for unpaired items and face counts
	Cardc  string // canonical value; "" when absent
	Forge  string
}

// Key is "<face>/<item>/<field>".
func (r Row) Key() string { return fmt.Sprintf("%d/%s/%s", r.Face, r.Item, r.Field) }

// Sig is the first 8 hex digits of sha256(Cardc + "\x00" + Forge).
func (r Row) Sig() string {
	h := sha256.Sum256([]byte(r.Cardc + "\x00" + r.Forge))
	return hex.EncodeToString(h[:])[:8]
}

// Shape is the family of rows about items and faces rather than params.
const Shape = "shape"

// familyKeys is the compared keys of each family; a key in no row is never compared.
var familyKeys = []struct {
	family string
	keys   []string
}{
	{"targets", []string{"ValidTgts", "TgtZone", "TargetMin", "TargetMax", "TargetType", "TargetUnique"}},
	{"who", []string{"Defined", "ValidCards", "ValidPlayers"}},
	{"amounts", []string{"Amount", "ChangeNum", "CharmNum", "ChoiceAmount", "CounterNum", "CounterType", "DigNum", "LifeAmount", "MinCharmNum", "Num", "NumAtt", "NumCards", "NumDef", "NumDmg", "NumTurns", "Produced"}},
	{"zones", []string{"Origin", "Destination", "ChangeType"}},
	{"cost", []string{"Cost", "SorcerySpeed", "ActivationLimit", "ActivationZone"}},
	{"trigger", []string{"ValidCard", "ValidPlayer", "ValidSource", "ValidTarget", "ValidActivatingPlayer", "Phase", "TriggerZones", "CombatDamage", "FirstTime", "Number"}},
	{"static", []string{"Affected", "AffectedZone", "AddPower", "AddToughness", "AddKeyword", "ValidAttacker", "Activator", "Type", "SetMaxHandSize", "MayPlay", "MayPlayWithoutManaCost", "Caster", "ValidSA", "EffectZone"}},
	{"repl", []string{"ActiveZones", "Layer", "ReplacementResult"}},
	{"cond", []string{"CheckSVar", "SVarCompare", "IsPresent", "PresentCompare", "PresentZone", "PlayerTurn", "ConditionCheckSVar", "ConditionSVarCompare", "ConditionPresent", "ConditionCompare"}},
}

// Families lists the family names, in table order, then Shape.
func Families() []string {
	out := make([]string, 0, len(familyKeys)+1)
	for _, f := range familyKeys {
		out = append(out, f.family)
	}
	return append(out, Shape)
}

// sortedKeys is every compared key with its family, in key order.
var sortedKeys, familyOf = func() ([]string, map[string]string) {
	m := map[string]string{}
	var ks []string
	for _, f := range familyKeys {
		for _, k := range f.keys {
			if _, dup := m[k]; !dup {
				ks = append(ks, k)
			}
			m[k] = f.family
		}
	}
	sort.Strings(ks)
	return ks, m
}()

// item is one comparable piece of a face.
type item struct {
	key    string
	line   map[string]string // line-level params; nil for A: and K: items
	chain  []*cards.SA       // body chain, Cleanup dropped
	svars  map[string]string
	exists bool
}

func chainNodes(sa *cards.SA) []*cards.SA {
	var out []*cards.SA
	for ; sa != nil; sa = sa.Sub {
		if sa.API != "Cleanup" {
			out = append(out, sa)
		}
	}
	return out
}

func chainOf(sa *cards.SA) string {
	var apis []string
	for _, n := range chainNodes(sa) {
		apis = append(apis, n.API)
	}
	return strings.Join(apis, ">")
}

func items(f *cards.Face) []item {
	var out []item
	for _, k := range f.Keywords {
		out = append(out, item{key: "K:" + k})
	}
	for _, a := range f.Abilities {
		out = append(out, item{key: "A:" + a.Kind + ":" + chainOf(a), chain: chainNodes(a)})
	}
	for _, t := range f.Triggers {
		out = append(out, item{key: "T:" + t.Mode + ":" + chainOf(t.Effect), line: t.Params, chain: chainNodes(t.Effect)})
	}
	for _, s := range f.Statics {
		out = append(out, item{key: "S:" + s.Mode, line: s.Params})
	}
	for _, r := range f.Repls {
		out = append(out, item{key: "R:" + r.Event + ":" + chainOf(r.With), line: r.Params, chain: chainNodes(r.With)})
	}
	return out
}

// grouped is a face's items by key, in order of appearance.
func grouped(f *cards.Face) map[string][]item {
	m := map[string][]item{}
	for _, it := range items(f) {
		it.svars = f.SVars
		m[it.key] = append(m[it.key], it)
	}
	return m
}

// Card compares cardc's definition of one card with Forge's. Rows come out in a
// fixed order: face, then item key, then field.
func Card(name string, cardc, forge *cards.Card) []Row {
	if len(cardc.Faces) != len(forge.Faces) {
		return []Row{{Card: name, Item: "faces", Field: "-", Family: Shape,
			Cardc: strconv.Itoa(len(cardc.Faces)), Forge: strconv.Itoa(len(forge.Faces))}}
	}
	var rows []Row
	for fi := range cardc.Faces {
		gc, gf := grouped(cardc.Faces[fi]), grouped(forge.Faces[fi])
		keySet := map[string]bool{}
		for k := range gc {
			keySet[k] = true
		}
		for k := range gf {
			keySet[k] = true
		}
		keys := make([]string, 0, len(keySet))
		for k := range keySet {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		for _, k := range keys {
			a, b := gc[k], gf[k]
			for i := 0; i < len(a) || i < len(b); i++ {
				id := k + "#" + strconv.Itoa(i)
				switch {
				case i >= len(b):
					rows = append(rows, Row{Card: name, Face: fi, Item: id, Field: "-", Family: Shape, Cardc: "present"})
				case i >= len(a):
					rows = append(rows, Row{Card: name, Face: fi, Item: id, Field: "-", Family: Shape, Forge: "present"})
				default:
					rows = append(rows, pair(name, fi, id, a[i], b[i])...)
				}
			}
		}
	}
	return rows
}

func pair(name string, face int, id string, c, f item) []Row {
	var rows []Row
	emit := func(field, key string, cv, fv string, cs, fs map[string]string) {
		cc, ff := canonKey(key, cv, cs), canonKey(key, fv, fs)
		if cc != ff {
			rows = append(rows, Row{Card: name, Face: face, Item: id, Field: field, Family: familyOf[key], Cardc: cc, Forge: ff})
		}
	}
	for _, k := range sortedKeys {
		cv, cok := c.line[k]
		fv, fok := f.line[k]
		if cok || fok {
			emit(k, k, cv, fv, c.svars, f.svars)
		}
	}
	for n := 0; n < len(c.chain) || n < len(f.chain); n++ {
		var cp, fp map[string]string
		if n < len(c.chain) {
			cp = c.chain[n].Params
		}
		if n < len(f.chain) {
			fp = f.chain[n].Params
		}
		tgt := cp["ValidTgts"] != "" || fp["ValidTgts"] != ""
		for _, k := range sortedKeys {
			cv, cok := cp[k]
			fv, fok := fp[k]
			if !cok && !fok {
				continue
			}
			if tgt && (k == "TargetMin" || k == "TargetMax") {
				if !cok {
					cv = "1"
				}
				if !fok {
					fv = "1"
				}
			}
			emit(fmt.Sprintf("[%d]%s", n, k), k, cv, fv, c.svars, f.svars)
		}
	}
	return rows
}
