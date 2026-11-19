package dirac

import "github.com/adams-shaun/gorge/dirac/kop"

// api Venture (CR 701.49; gorge effects/venture.go effVenture) over
// act_initiative.go's marker. ExtVenture ventures its player into the
// dungeon; the initiative's OpTakeInitiative ventures into Undercity
// (undercity true in venture). With no dungeon a plain venture offers every
// dungeon card of the table but Undercity (prep.dungeons, key order: gorge
// dungeonCandidates): one candidate is entered without asking, several ask
// the venturer (raDungeon, gorge's "dungeon" KChoose, ChDungeon: mode the
// candidate's index). The room choice (raRoom) and the completion are
// act_initiative.go's, read off the card of the player's dungeon object.

// VentureStats counts the venture asks of a run; oracle tests only.
type VentureStats struct {
	DungeonAsks int // dungeon choices posed (raDungeon)
	RoomAsks    int // room choices posed (raRoom)
	LandAsks    int // lands a Play picked whose entry may ask (freeLandAsk)
}

// DebugVentureStats is the test hook; nil in normal play.
var DebugVentureStats *VentureStats

// DebugVentureNoLast (TestVentureDungeonMutation): the dungeon ask leaves out
// its last candidate.
var DebugVentureNoLast bool

// ChDungeon is gorge's venture "dungeon" KChoose: mode j the j-th candidate
// dungeon in key order.
const ChDungeon ChoiceKind = 205

// dungeonChoose is CR 701.49a's dungeon choice for p, who has none; false:
// the dungeon ask is posed.
func (s *State) dungeonChoose(k *prep, p uint8, undercity bool) bool {
	if undercity {
		s.dungeonEnter(k, p, k.gen.dungeon)
		return true
	}
	switch n := len(k.dungeons); n {
	case 0:
		return true
	case 1:
		s.dungeonEnter(k, p, k.dungeons[0])
		return true
	}
	if st := DebugVentureStats; st != nil {
		st.DungeonAsks++
	}
	s.res.ask, s.res.askP = raDungeon, p
	return false
}

// ventureExt is ExtVenture: the player Obj ventures into the dungeon (gorge
// effVenture, always the resolving controller).
//
//kmanifest:ext ExtVenture
func (s *State) ventureExt(k *prep, in *kop.Instr) (ok, done bool) {
	p := s.playerRef(k, in.Obj)
	if p >= MaxSeats {
		return true, true
	}
	return s.venture(k, p, false), true
}

// dungeonAskOptions poses the dungeon ask: one option per candidate.
//
//kmanifest:askopts raDungeon
func (s *State) dungeonAskOptions(k *prep, d *Decision) {
	d.Sub = ChDungeon
	n := len(k.dungeons)
	if DebugVentureNoLast {
		n--
	}
	for j := 0; j < n; j++ {
		d.add(MakeAction(AkChoose, 0, 0, uint8(j)))
	}
}

// dungeonAskAnswer enters the answered dungeon (an index past the candidates
// takes the first, gorge's stand-in), then the resolution goes on.
//
//kmanifest:askapply raDungeon
func (s *State) dungeonAskAnswer(k *prep, a Action) {
	p := s.res.askP
	j := int(a.Mode())
	if j >= len(k.dungeons) {
		j = 0
	}
	s.res.ask = raNone
	s.dungeonEnter(k, p, k.dungeons[j])
	s.ventureResume(k)
}
