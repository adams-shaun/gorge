package kgen

import (
	"fmt"
	"strings"

	"github.com/adams-shaun/gorge/engine/cards"
)

// api TakeInitiative (CR 726; gorge effects/initiative.go
// effTakeInitiative) and the Undercity dungeon it ventures into (CR 309,
// 701.49; gorge effects/venture.go). The dungeon is a pool token entry
// (undercityKey, the token script's stem) lowered by lowerDungeonEntry: one
// latent ability per room in K:Dungeon order, the room SVar's chain, and
// the NextRoom$ arrows as CardDef.DungeonNext data.

// undercityKey is the Undercity token script's key (gorge Game.Tokens).
const undercityKey = "undercity"

// takeInitiative lowers TakeInitiative: the resolving controller takes the
// initiative and ventures into Undercity. Defined$ other than You refuses.
func (lc *lowerCtx) takeInitiative(ps *params) error {
	if d, ok := ps.get("Defined"); ok && strings.TrimSpace(d) != "You" {
		return refuse("TakeInitiative Defined$ %q", d)
	}
	c, ok := lc.p.reg.Token(undercityKey)
	if !ok || len(c.Faces) == 0 {
		return refuse("no %q dungeon script in the corpus", undercityKey)
	}
	lc.out.Tokens = append(lc.out.Tokens, undercityKey)
	// the rooms' own tokens (Stash's Treasure, Catacombs' Skeleton): the
	// pool lowers only the tokens a card names, so the carrier names them,
	// in room order (SVars is a map)
	for _, r := range dungeonRooms(c.Faces[0]) {
		for _, part := range strings.Split(c.Faces[0].SVars[r], "|") {
			k, v, ok := strings.Cut(strings.TrimSpace(part), "$")
			if ok && strings.TrimSpace(k) == "TokenScript" {
				lc.out.Tokens = append(lc.out.Tokens, strings.TrimSpace(v))
			}
		}
	}
	lc.emit(lInstr{Op: "TakeInitiative", Obj: lRef{Kind: rYou}, CS: undercityKey})
	return nil
}

// dungeonRooms is the face's printed K:Dungeon room order.
func dungeonRooms(f *cards.Face) []string {
	list, ok := f.KeywordParam("Dungeon")
	if !ok {
		return nil
	}
	var out []string
	for _, r := range strings.Split(list, ",") {
		if r = strings.TrimSpace(r); r != "" {
			out = append(out, r)
		}
	}
	return out
}

// lowerDungeonEntry lowers a dungeon token script: no record beyond the
// name (a dungeon is no permanent), each room a latent ability.
func lowerDungeonEntry(p *pool, k string) *lCard {
	out := &lCard{Name: k, Token: true}
	c, ok := p.reg.Token(k)
	lc := &lowerCtx{p: p, name: k, card: c, out: out, mode: -1, known: p.knownSubtype}
	if !ok || len(c.Faces) != 1 {
		lc.refuseNode("card", "dungeon script %q", k)
		return out
	}
	lc.face = c.Faces[0]
	rooms := dungeonRooms(lc.face)
	if len(rooms) == 0 || len(rooms) > 15 {
		lc.refuseNode("K:Dungeon", "%d rooms", len(rooms))
		return out
	}
	idx := map[string]int{}
	for i, r := range rooms {
		idx[r] = i
	}
	out.DungeonNext = make([]uint8, len(rooms))
	for i, r := range rooms {
		sa := cards.ResolveSVar(lc.face.SVars, r)
		if sa == nil {
			lc.refuseNode("K:Dungeon", "room SVar %q", r)
			return out
		}
		var nx []string
		for _, n := range strings.Split(sa.Params["NextRoom"], ",") {
			if n = strings.TrimSpace(n); n != "" {
				nx = append(nx, n)
			}
		}
		if len(nx) > 2 {
			lc.refuseNode("K:Dungeon", "room %q: %d arrows", r, len(nx))
			return out
		}
		for j, n := range nx {
			ni, ok := idx[n]
			if !ok {
				lc.refuseNode("K:Dungeon", "room %q: arrow %q", r, n)
				return out
			}
			out.DungeonNext[i] |= uint8(ni+1) << (4 * j)
		}
		ab := lc.newLatent("Room")
		lc.guard(fmt.Sprintf("Room %s", r), func() error {
			return lc.withAbility(ab, func() error {
				ps := newParams(sa.Params)
				ps.consume("RoomName", "NextRoom", "SpellDescription")
				return lc.chain(sa, ps)
			})
		})
	}
	return out
}
