package kgen

import (
	"strconv"
	"strings"

	"github.com/adams-shaun/gorge/dirac/kop"
	"github.com/adams-shaun/gorge/engine/cards"
)

// Rooms (CR 709.5, gorge rules face_rooms.go). A two-door Room
// is a split card whose halves are both Enchantment Rooms (kop.FaceRoom).
// Either door is cast; the cast door is unlocked as the Room enters and the
// other stays locked until its cost is paid (the unlock special action). A
// Room's door state lives in its CardID: kgen emits one pool entry per
// battlefield state (face f showing, door mask U unlocked) beside the face
// entries, keyed "room:<card>#<f><U>", carrying exactly the lines that
// function in that state (gorge's room-aware scans walk the FaceIdx door
// while it is unlocked and the other door once unlocked), the record of
// face f (gorge derives colours and types from o.Face(),
// effects.ColorMaskOf) and the mana value of the unlocked doors (gorge
// state/mana_value.go roomManaValue, CR 709.5). The plain faces serve the
// states (f, {f}) a cast Room enters with.

// roomAllLines is a plain door def's lCard.RoomSplit: all its lines are
// its own door's.
const roomAllLines = 1 << 20

// roomKeyPrefix prefixes a Room state entry's pool name.
const roomKeyPrefix = "room:"

// roomFace reports whether f is a Room door (gorge rules isRoomFace).
func roomFace(f *cards.Face) bool {
	return f != nil && f.IsEnchantment() && f.IsRoom()
}

// roomKeys is card name's six state keys in kop.RoomState order
// (Faces[2:]).
func roomKeys(name string) []string {
	var out []string
	for f := uint8(0); f < 2; f++ {
		for _, m := range [3]uint8{0, 1 << (1 - f), 3} {
			out = append(out, roomKeyPrefix+name+"#"+strconv.Itoa(int(f))+strconv.Itoa(int(m)))
		}
	}
	return out
}

// parseRoomKey splits a Room state key into card name, face and mask.
func parseRoomKey(k string) (string, int, int, bool) {
	rest, ok := strings.CutPrefix(k, roomKeyPrefix)
	if !ok {
		return "", 0, 0, false
	}
	i := strings.LastIndexByte(rest, '#')
	if i < 0 || len(rest)-i != 3 {
		return "", 0, 0, false
	}
	f, m := int(rest[i+1]-'0'), int(rest[i+2]-'0')
	if f < 0 || f > 1 || m < 0 || m > 3 || m == 1<<f {
		return "", 0, 0, false
	}
	return rest[:i], f, m, true
}

// addManaCost adds b's symbols to a (CR 709.5: a Room's mana value is the
// sum of its unlocked doors'; only the MV is read off a state def).
func addManaCost(a, b manaCost) manaCost {
	a.Generic += b.Generic
	for i := range a.Pips {
		a.Pips[i] += b.Pips[i]
	}
	a.Hybrid = append(append([]string(nil), a.Hybrid...), b.Hybrid...)
	a.C += b.C
	a.X, a.XX = a.X || b.X, a.XX || b.XX
	return a
}

// refuseRoomDoors refuses the Room door shapes the state defs cannot
// express: a non-mana activated ability (gorge maps an unlocked other
// door's activation by name, not by the FaceIdx face's index), and a
// cost modifier on the unlock action (Static.Unlock, Inquisitive Glimmer's
// shape: the unlock pays the printed cost only), and a hybrid door cost
// (the unlock's pool pricing reads no hybrid symbol).
//
// A body that names another SVar is refused when gorge would read the name
// on the shown face's table. A trigger resolution is not that shape: the
// kernel compiles each unlocked door's program with lc.face set to that door
// (lowerRoomEntry's lowerLines loop), and every body read on the chain
// (effectStaticBodies, emblem, effectPrevent, the replacement walk) resolves
// the name in lc.face.SVars -- the door's own table, exactly what gorge
// binds when it queues the trigger (rules/rooms.go queueUnlockTriggers:
// TriggerSVars: face.SVars). So StaticAbilities$, Triggers$ and
// ReplacementEffects$ on a trigger's Execute$/SubAbility$ chain are safe.
//
// A STATIC's own AddAbility$/AddStaticAbility$/AddTrigger$/AddReplacementEffects$
// stays refused (Greenhouse): gorge's layer walk binds an unlocked other
// door's static to the shown face's table (rules/layers.go:187), a gorge
// defect briefed separately. The lifted names are therefore allowed only for
// bodies reached from a trigger, and the static's four stay refused
// everywhere.
func (lc *lowerCtx) refuseRoomDoors(c *cards.Card) {
	named := [...]string{"StaticAbilities$", "Triggers$", "AddAbility$", "AddTrigger$", "AddStaticAbility$", "ReplacementEffects$", "AddReplacementEffects$"}
	staticNamed := [...]string{"AddAbility$", "AddTrigger$", "AddStaticAbility$", "AddReplacementEffects$"}
	for _, f := range c.Faces {
		triggerBodies := roomTriggerBodies(f)
		hit := "" // the least key found: map order must not reach the reason
		for name, b := range f.SVars {
			keys := named[:]
			if triggerBodies[name] {
				keys = staticNamed[:]
			}
			for _, k := range keys {
				if strings.Contains(b, k) && (hit == "" || k < hit) {
					hit = k
				}
			}
		}
		for _, s := range f.Statics {
			for k, v := range s.Params {
				b := k + "$ " + v
				for _, n := range named {
					if strings.Contains(b, n) && (hit == "" || n < hit) {
						hit = n
					}
				}
			}
		}
		if hit != "" {
			lc.refuseNode("card", "Room door line naming an SVar body (%s)", hit)
			return
		}
	}
	for _, f := range c.Faces {
		// the unlock pays the door's printed cost from the pool; the kernel
		// prices no hybrid symbol there (gorge would offer it)
		if mc, err := parseManaCost(f.ManaCost); err == nil && len(mc.Hybrid) != 0 {
			lc.refuseNode("card", "Room door hybrid cost")
			return
		}
		for _, a := range f.Abilities {
			if a.Kind == "AB" && a.API != "Mana" {
				lc.refuseNode("card", "Room door with a non-mana activated ability")
				return
			}
		}
	}
}

// roomTriggerBodies is the SVar names a face's triggers execute, following
// their SubAbility$ chains: the bodies a trigger resolves with its own door's
// table. A trigger line that names another SVar body is a trigger-chain body;
// one only a static names is not (its read is the shown-face layer walk).
func roomTriggerBodies(f *cards.Face) map[string]bool {
	out := map[string]bool{}
	var mark func(string)
	mark = func(name string) {
		name = strings.TrimSpace(name)
		if name == "" || out[name] {
			return
		}
		body, ok := f.SVars[name]
		if !ok {
			return
		}
		out[name] = true
		// Follow every SVar reference a body may resolve: the plain and
		// branch SubAbility$ spellings, a trigger's Execute$, a charm's
		// Choices$ and a replacement's ReplaceWith$. Each resolves with the
		// same door table the trigger binds, so its body is on the chain.
		for _, part := range strings.Split(body, "|") {
			k, v, ok := strings.Cut(strings.TrimSpace(part), "$")
			if !ok {
				continue
			}
			k, v = strings.TrimSpace(k), strings.TrimSpace(v)
			if k != "Execute" && k != "Choices" && k != "ReplaceWith" && !strings.HasSuffix(k, "SubAbility") {
				continue
			}
			for _, n := range strings.FieldsFunc(v, func(r rune) bool {
				return r == ',' || r == '&' || r == ' ' || r == '\t'
			}) {
				mark(n)
			}
		}
	}
	for _, t := range f.Triggers {
		mark(t.ParamStr(cards.PKExecute))
	}
	return out
}

// lowerRoomEntry lowers Room state key k as its own pool entry.
func lowerRoomEntry(p *pool, k string) *lCard {
	out := &lCard{Name: k}
	name, f, m, ok := parseRoomKey(k)
	c := p.byName[name]
	lc := &lowerCtx{p: p, name: k, card: c, out: out, mode: -1, known: p.knownSubtype}
	if !ok || c == nil || faceLayout(c, false) != kop.FaceRoom {
		lc.refuseNode("card", "room key %q", k)
		return out
	}
	out.FaceMode, out.Face = kop.FaceRoom, f
	out.Faces = faceNames(name, c, kop.FaceRoom)
	lc.face = c.Faces[f]
	lc.lowerRecord()
	lc.lowerKeywords()
	var cost manaCost
	for d := 0; d < 2; d++ {
		if m&(1<<d) != 0 {
			mc, err := parseManaCost(c.Faces[d].ManaCost)
			if err != nil {
				lc.refuseNode("ManaCost", "%v", err)
			}
			cost = addManaCost(cost, mc)
		}
	}
	out.Cost, out.CostEmpty = cost, false
	if m&(1<<f) != 0 {
		lc.lowerLines(true, true)
	}
	out.RoomSplit = len(out.Abilities)
	if o := 1 - f; m&(1<<o) != 0 {
		lc.face = c.Faces[o]
		lc.lowerLines(false, true) // gorge projects another face's line as 255
		lc.face = c.Faces[f]
	}
	lc.lowerTail()
	lc.refuseRoomDoors(c)
	return out
}
