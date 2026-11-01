package policynet

import (
	"fmt"
	"strings"

	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/state"
	"github.com/adams-shaun/gorge/engine/view"
)

// FeatureSet selects the state/option encoding a model was trained under
// (ticket pn12, the feature axis). FeaturesV1 is the pinned encoder of
// policynet.go/option.go, byte for byte: every existing checkpoint, golden and
// digest reads it. The other sets are ADDITIVE: they emit the v1 encoding
// unchanged and then append further hashed rows into the same shared
// TableRows space, so the model geometry (dense widths, slot widths, InW) is
// identical for every set and only the encoder hash differs.
//
//   - FeaturesMZ ("mz"): MageZero-style per-card properties for both
//     battlefields and the viewer's hand (types, current P/T, keywords,
//     tapped, summoning sick, can-attack/can-block, counters, attached-to,
//     damage, attacking), per-seat available (untapped) mana, the stack's
//     spell APIs, and the option-side pn03 bits (the option card's spell API;
//     the decision-kind "other" routed to a hashed id instead of the v1 slot
//     that collides with the Land bit). Read from the redacted view only, so
//     a seat can encode it: checkpointable and playable.
//   - FeaturesMZOppHand ("mz-opphand"): FeaturesMZ plus the opponent's hand
//     (a Diag the label corpus records). DIAGNOSTIC ONLY: it reads hidden
//     information, so WriteCheckpoint refuses it and no seat can load it.
//   - FeaturesMZOracle ("mz-oracle"): FeaturesMZOppHand plus the next draws
//     of both libraries. DIAGNOSTIC ONLY, same refusal.
//   - FeaturesEntity ("entity", ticket pn14): FeaturesMZ plus one ENTITY
//     vector per visible card (entity.go: State.Cards, Option.EntA/EntB).
//     Unlike every set above it changes the model geometry: an entity model
//     carries a per-card encoder, a pooled projection into the state trunk
//     and 2·EntK extra hidden inputs per option (Model.EntK > 0). Read from
//     the redacted view only: checkpointable (schema version 4) and playable.
//   - FeaturesMZOwnLib ("mz-ownlib"): FeaturesMZ plus the seat's HONEST
//     remaining own-library composition (ownlib.go): one
//     "mz|ownlibleft|<name>" row per card name still in the library, valued
//     copies/library size, and "mz|ownlib|land" = the land fraction; when the
//     composition is not derivable from the view, one "mz|ownlib|unknown"
//     row instead. Folded by view.OwnLibrary off the seat's own deck list and
//     what it sees -- never the view's own Library list -- so it is
//     checkpointable and playable. Same geometry as mz (sparse rows only).
//   - FeaturesMZGrave ("mz-grave"): FeaturesMZ plus per-card graveyard
//     property tokens for both seats (graveyard.go): types, mana value, spell
//     API, keywords and creature P/T, under "mz|gy-me|" / "mz|gy-opp|". Read
//     from the public graveyards of the redacted view: checkpointable and
//     playable. Same geometry as mz (sparse rows only).
type FeatureSet uint8

const (
	FeaturesV1 FeatureSet = iota
	FeaturesMZ
	FeaturesMZOppHand
	FeaturesMZOracle
	FeaturesEntity
	// FeaturesMZOwnLib is appended after every earlier set: the ordinals are
	// part of what a checkpoint's feature set names, never renumbered.
	FeaturesMZOwnLib
	// FeaturesMZGrave is appended after FeaturesMZOwnLib, same rule.
	FeaturesMZGrave
)

var featureSetNames = [...]string{"v1", "mz", "mz-opphand", "mz-oracle", "entity", "mz-ownlib", "mz-grave"}

// String is the flag spelling.
func (fs FeatureSet) String() string {
	if int(fs) < len(featureSetNames) {
		return featureSetNames[fs]
	}
	return fmt.Sprintf("featureset(%d)", uint8(fs))
}

// ParseFeatureSet maps a flag spelling onto a FeatureSet ("" is v1).
func ParseFeatureSet(s string) (FeatureSet, error) {
	if s == "" {
		return FeaturesV1, nil
	}
	for i, n := range featureSetNames {
		if n == s {
			return FeatureSet(i), nil
		}
	}
	return 0, fmt.Errorf("unknown feature set %q (want %s)", s, strings.Join(featureSetNames[:], ", "))
}

// Diagnostic reports whether the set reads hidden information (the opponent's
// hand or future draws). A diagnostic model is a measurement vehicle only:
// it is never checkpointed and never played by a seat.
func (fs FeatureSet) Diagnostic() bool { return fs == FeaturesMZOppHand || fs == FeaturesMZOracle }

// Diag is the hidden information a diagnostic feature set reads: recorded by
// the search teacher (-label-extras) (LabelExtras) and never available to a
// seat.
type Diag struct {
	OppHand       []view.CardView
	OppLibraryTop []string
	OwnLibraryTop []string
}

// SplitOmniscientView turns an omniscient projection (view.ProjectFor with
// view.Omniscient, viewer = seat) into what a diagnostic training record
// holds: the seat's view with every OTHER seat's hand removed, plus a Diag
// whose OppHand is those hands concatenated in player order -- the order
// the search teacher's label extras record them in. The state encoders read
// only the seat's own hand, so the removal changes no base feature; it keeps
// the returned view's hands shaped like a Seat projection's. Library tops are
// not in any view (an omniscient projection never shows library order), so
// the Diag carries none: it is complete for FeaturesMZOppHand only.
//
// v is not modified: the Players slice is copied before a hand is cleared.
func SplitOmniscientView(v view.View, seat state.PlayerID) (view.View, *Diag) {
	diag := &Diag{OppHand: []view.CardView{}}
	players := make([]view.PlayerView, len(v.Players))
	copy(players, v.Players)
	for i := range players {
		if players[i].ID == seat {
			continue
		}
		diag.OppHand = append(diag.OppHand, players[i].Hand...)
		players[i].Hand = nil
	}
	v.Players = players
	return v, diag
}

// EncoderHashFor is the encoder digest a checkpoint of feature set fs
// carries. FeaturesV1 is EncoderHash() unchanged (every existing checkpoint
// keeps loading); every other set digests the v1 hash plus its own
// name and token-format version, so a checkpoint cannot be read under the
// wrong feature set.
func EncoderHashFor(fs FeatureSet) uint64 {
	if fs == FeaturesV1 {
		return EncoderHash()
	}
	if fs == FeaturesEntity {
		return encoderHashWith(fs, entityHashSuffix())
	}
	return encoderHashWith(fs, "")
}

// EncoderHashEntityV1 is the encoder digest of the format-1 (68-wide) entity
// encoding: the hash every network and training corpus recorded before the
// relational slots existed. It is not a checkpointable feature set for
// training (FeaturesForHash refuses it), only a load path.
func EncoderHashEntityV1() uint64 { return encoderHashWith(FeaturesEntity, entityHashSuffixV1()) }

// encoderHashWith is EncoderHashFor's body with the entity suffix supplied by
// the caller, so the format-1 and format-2 entity hashes share one string
// shape.
func encoderHashWith(fs FeatureSet, entSuffix string) uint64 {
	s := fmt.Sprintf("gorge-policynet-features\x1fbase=%#016x\x1fset=%s\x1fmz-format=1\x1fpins=%d,%d",
		EncoderHash(), fs, hashID("mz|bf-opp|canblock"), hashID("mz|me|avail"))
	s += entSuffix
	if fs == FeaturesMZOwnLib {
		s += ownLibHashSuffix()
	}
	if fs == FeaturesMZGrave {
		s += graveHashSuffix()
	}
	h := uint64(14695981039346656037)
	for i := 0; i < len(s); i++ {
		h ^= uint64(s[i])
		h *= 1099511628211
	}
	return h
}

// FeaturesForHash maps a checkpoint's encoder hash back to the non-diagnostic
// feature set it was written under.
func FeaturesForHash(h uint64) (FeatureSet, bool) {
	for _, fs := range []FeatureSet{FeaturesV1, FeaturesMZ, FeaturesEntity, FeaturesMZOwnLib, FeaturesMZGrave} {
		if EncoderHashFor(fs) == h {
			return fs, true
		}
	}
	return 0, false
}

// Features is the feature set the scorer's model was trained under; a seat
// encodes with EncodeStateWith/EncodeOptionWith of this set.
func (sc *Scorer) Features() FeatureSet { return sc.m.Features }

// EncodeStateWith is EncodeState under feature set fs. FeaturesV1 is exactly
// EncodeState. diag is read only by the diagnostic sets (nil = none known).
func EncodeStateWith(fs FeatureSet, v view.View, seat state.PlayerID, diag *Diag) State {
	return encodeStateWith(nil, fs, &v, seat, diag)
}

// encodeStateWith is EncodeStateWith into buf's storage (fresh storage for
// a nil buf).
func encodeStateWith(buf *StateBuf, fs FeatureSet, v *view.View, seat state.PlayerID, diag *Diag) State {
	st := encodeState(buf, v, seat)
	if fs == FeaturesV1 {
		return st
	}
	// The mz rows are appended straight after the v1 bag, then the whole
	// bag is stably sorted by row.
	sk := &sink{fs: st.Sparse}
	mzState(*v, seat, sk)
	if fs == FeaturesMZOwnLib {
		ownLibState(*v, seat, sk.str)
	}
	if fs == FeaturesMZGrave {
		graveyardState(*v, seat, sk.str)
	}
	if fs.Diagnostic() && diag != nil {
		mzDiag(*v, seat, fs, diag, sk)
	}
	st.Sparse = sk.fs
	stableSortByRow(st.Sparse)
	buf.keepSparse(st.Sparse)
	if fs == FeaturesEntity {
		st.Cards = entityCardList(buf, v, seat)
	}
	return st
}

// EncodeOptionWith is EncodeOption under feature set fs. FeaturesV1 is
// exactly EncodeOption. Every other set (a) routes a decision kind outside
// decision.Kinds' named slots to a hashed "mz|deckind|other" id instead of
// the v1 "other" slot, which sits one past the 13-slot block ON the Land
// type bit (the pn03 finding: slot 24+len(decision.Kinds) == typeOffset),
// and (b) adds the option card's spell API as a hashed id (pn03's counter
// vs. other-instant signal).
func EncodeOptionWith(fs FeatureSet, v view.View, seat state.PlayerID, d decision.Kind, o decision.Option, idx, n int) Option {
	return withEncoder(&v, seat, func(x *OptionEncoder) Option { return x.Encode(fs, d, o, idx, n) })
}

// dropSlot removes the first slot at row r (a v1 "other" slot is set once).
func dropSlot(slots []Feature, r uint16) []Feature {
	for i, f := range slots {
		if f.Row == r {
			return append(slots[:i:i], slots[i+1:]...)
		}
	}
	return slots
}

// mzBucket clamps a small integer into 0..hi for a categorical token.
func mzBucket(v int32, hi int32) string {
	if v < 0 {
		v = 0
	}
	if v > hi {
		v = hi
	}
	return formatInt(int64(v))
}

// hasKeyword reports whether some keyword of cv equals keywordBits[kw]
// (case-insensitive).
func hasKeyword(cv *view.CardView, kw int) bool {
	for _, k := range cv.Keywords {
		if keywordBit(k) == kw {
			return true
		}
	}
	return false
}

// seatAvailable is a seat's floating pool plus its available (tappable)
// mana, in the fixed WUBRG C order — public information for every seat
// (view.PlayerView.Pool/Available).
func seatAvailable(pv *view.PlayerView) int32 {
	var t int32
	if pv.Mana.OK {
		for c := range poolKeys {
			t += pv.Mana.Pool[c] + pv.Mana.Available[c]
		}
		return t
	}
	for _, k := range poolKeys {
		t += pv.Pool[k] + pv.Available[k]
	}
	return t
}

// mzState emits the MZ per-card and per-seat tokens in a fixed order: the
// viewer's battlefield, every other battlefield (seat order), the viewer's
// hand, the stack, then the seat aggregates. Each token is hashed as it is
// streamed (hashstream.go): the row is hashID of the token string the
// comment beside it spells, which is never built.
func mzState(v view.View, seat state.PlayerID, sk *sink) {
	var me *view.PlayerView
	var othersBuf [8]*view.PlayerView
	others := othersBuf[:0]
	for i := range v.Players {
		if v.Players[i].ID == seat {
			me = &v.Players[i]
		} else {
			others = append(others, &v.Players[i])
		}
	}
	type agg struct{ canAttack, canAttackPower, canBlock, canBlockTough, flyers int32 }
	perm := func(z string, cv *view.CardView, a *agg) {
		zh := fnvAdd(fnvAdd(fnvAdd(fnvOffset, "mz|"), z), "|") // "mz|<z>|"
		eachField(cv.Types, func(t string) { sk.h(fnvAdd(fnvAdd(zh, "type|"), t), 1) })
		creature := hasType(cv, cards.TypeCreature)
		if cv.Tapped {
			sk.h(fnvAdd(zh, "tapped"), 1)
		} else {
			sk.h(fnvAdd(zh, "untapped"), 1)
		}
		if cv.SummonSick {
			sk.h(fnvAdd(zh, "sick"), 1)
		}
		if cv.Damage > 0 {
			sk.h(fnvAdd(zh, "damaged"), 1)
		}
		if cv.Attacking {
			sk.h(fnvAdd(zh, "attacking"), 1)
		}
		if len(cv.BlockedBy) > 0 {
			sk.h(fnvAdd(zh, "blocked"), 1)
		}
		kwh := fnvAdd(zh, "kw|")
		for _, k := range cv.Keywords {
			sk.h(fnvAddLower(kwh, k), 1) // "mz|<z>|kw|<lower k>"
		}
		if len(cv.Counters) > 0 {
			ctrh := fnvAdd(zh, "ctr|")
			eachCounterSorted(cv.Counters, func(k string, n int32) { sk.h(fnvAdd(ctrh, k), float32(n)) })
		}
		if cv.AttachedTo != 0 {
			sk.h(fnvAdd(zh, "attached"), 1)
			if host, ok := battlefieldName(&v, cv.AttachedTo); ok {
				sk.h(fnvAdd(fnvAdd(zh, "attached|"), host), 1)
				sk.h(fnvAdd(fnvAdd(fnvAdd(fnvAdd(fnvOffset, "mz|"), cv.Name), "|on|"), host), 1)
			}
		}
		if !creature {
			return
		}
		p, t := mzClamp(cv.Power, 9), mzClamp(cv.Toughness-cv.Damage, 9)
		crh := fnvAdd(zh, "cr|")
		crp := fnvAddInt(fnvAdd(crh, "p"), p)
		sk.h(fnvAddInt(fnvAdd(crp, "|t"), t), 1)                                // "mz|<z>|cr|p<p>|t<t>"
		sk.h(crp, 1)                                                            // "mz|<z>|cr|p<p>"
		sk.h(fnvAddInt(fnvAdd(crh, "t"), t), 1)                                 // "mz|<z>|cr|t<t>"
		nh := fnvAdd(fnvAdd(fnvAdd(fnvAdd(fnvOffset, "mz|"), cv.Name), "|"), z) // "mz|<name>|<z>"
		sk.h(fnvAddInt(fnvAdd(fnvAddInt(fnvAdd(nh, "|p"), p), "|t"), t), 1)
		crkw := fnvAdd(crh, "kw|")
		for _, k := range cv.Keywords {
			sk.h(fnvAddLower(crkw, k), 1)
		}
		if hasKeyword(cv, kwFlying) {
			a.flyers++
		}
		canAttack := !cv.Tapped && (!cv.SummonSick || hasKeyword(cv, kwHaste)) && !hasKeyword(cv, kwDefender)
		if canAttack {
			sk.h(fnvAdd(zh, "canattack"), 1)
			sk.h(fnvAddInt(fnvAdd(zh, "canattack|p"), p), 1)
			a.canAttack++
			a.canAttackPower += cv.Power
		}
		if !cv.Tapped {
			sk.h(fnvAdd(zh, "canblock"), 1)
			sk.h(fnvAddInt(fnvAdd(zh, "canblock|t"), t), 1)
			a.canBlock++
			a.canBlockTough += cv.Toughness - cv.Damage
		}
	}
	var mine, theirs agg
	if me != nil {
		for i := range me.Battlefield {
			perm("bf-me", &me.Battlefield[i], &mine)
		}
	}
	for _, pv := range others {
		for i := range pv.Battlefield {
			perm("bf-opp", &pv.Battlefield[i], &theirs)
		}
	}
	var myAvail int32
	if me != nil {
		myAvail = seatAvailable(me)
		for i := range me.Hand {
			handCard("hand", &me.Hand[i], myAvail, sk)
		}
	}
	for _, sv := range v.Stack {
		if sv.Card == nil {
			continue
		}
		side := "opp"
		if sv.Card.Controller == seat {
			side = "me"
		}
		sh := fnvAdd(fnvAdd(fnvOffset, "mz|stack|"), side) // "mz|stack|<side>"
		sk.h(sh, 1)
		if sv.Card.SpellAPI != "" {
			sk.h(fnvAdd(fnvAdd(sh, "|api|"), sv.Card.SpellAPI), 1)
		}
		eachField(sv.Card.Types, func(t string) { sk.h(fnvAdd(fnvAdd(sh, "|type|"), t), 1) })
	}
	// Seat aggregates: numeric-valued rows (the embedding row times the count)
	// plus a bucketed one-hot, so both a linear and a threshold read exist.
	num := func(key string, val int32, hi int32) {
		kh := fnvAdd(fnvAdd(fnvOffset, "mz|"), key)
		sk.h(kh, float32(val))                                // "mz|<key>"
		sk.h(fnvAddInt(fnvAdd(kh, "="), mzClamp(val, hi)), 1) // "mz|<key>=<bucket>"
	}
	num("me|avail", myAvail, 8)
	var oppAvail int32
	for _, pv := range others {
		oppAvail += seatAvailable(pv)
	}
	num("opp|avail", oppAvail, 8)
	num("me|canattack-n", mine.canAttack, 6)
	num("me|canattack-power", mine.canAttackPower, 12)
	num("opp|canblock-n", theirs.canBlock, 6)
	num("opp|canblock-tough", theirs.canBlockTough, 12)
	num("opp|canattack-n", theirs.canAttack, 6)
	num("opp|canattack-power", theirs.canAttackPower, 12)
	num("me|canblock-n", mine.canBlock, 6)
	num("me|flyers", mine.flyers, 4)
	num("opp|flyers", theirs.flyers, 4)
	if me != nil && len(others) > 0 {
		diff := me.Life - others[0].Life
		sk.h(fnvAddInt(fnvAdd(fnvOffset, "mz|lifediff="), mzClamp(diff/3+7, 14)), 1)
		sk.h(fnvAddInt(fnvAdd(fnvOffset, "mz|opp|life="), mzClamp(others[0].Life/2, 12)), 1)
		sk.h(fnvAddInt(fnvAdd(fnvOffset, "mz|me|life="), mzClamp(me.Life/2, 12)), 1)
		// Lethal reads: can my untapped attackers' power alone kill through no
		// blocks; can theirs kill me.
		if mine.canAttackPower >= others[0].Life {
			sk.h(fnvAdd(fnvOffset, "mz|me|lethal-unblocked"), 1)
		}
		if theirs.canAttackPower >= me.Life {
			sk.h(fnvAdd(fnvOffset, "mz|opp|lethal-unblocked"), 1)
		}
	}
}

// battlefieldName is the name the attached-host lookup reads: the LAST
// battlefield card (players in order, each battlefield in order) with id --
// the old per-encode name map's last-write-wins value.
func battlefieldName(v *view.View, id state.ObjID) (string, bool) {
	for i := len(v.Players) - 1; i >= 0; i-- {
		bf := v.Players[i].Battlefield
		for j := len(bf) - 1; j >= 0; j-- {
			if bf[j].ID == id {
				return bf[j].Name, true
			}
		}
	}
	return "", false
}

// eachCounterSorted calls f for each counter in ascending key order (the
// sorted-keys walk the encoders always used), without allocating for up
// to eight kinds.
func eachCounterSorted(m map[string]int32, f func(string, int32)) {
	var buf [8]string
	keys := buf[:0]
	for k := range m {
		keys = append(keys, k)
	}
	// Insertion sort: the keys are distinct, so the order is the unique
	// ascending one, as sort.Strings leaves it.
	for i := 1; i < len(keys); i++ {
		for j := i; j > 0 && keys[j] < keys[j-1]; j-- {
			keys[j], keys[j-1] = keys[j-1], keys[j]
		}
	}
	for _, k := range keys {
		f(k, m[k])
	}
}

// handCard emits one hand card's tokens under zone tag z; avail is the
// holder's available mana (castability is mana-value-only, an approximation:
// colours are not checked).
func handCard(z string, cv *view.CardView, avail int32, sk *sink) {
	zh := fnvAdd(fnvAdd(fnvAdd(fnvOffset, "mz|"), z), "|") // "mz|<z>|"
	mv := int32(cardManaValue(cv))
	eachField(cv.Types, func(t string) { sk.h(fnvAdd(fnvAdd(zh, "type|"), t), 1) })
	sk.h(fnvAddInt(fnvAdd(zh, "mv"), mzClamp(mv, 8)), 1)
	instant := hasType(cv, cards.TypeInstant) || hasKeyword(cv, kwFlash)
	if instant {
		sk.h(fnvAdd(zh, "instant-speed"), 1)
	}
	land := hasType(cv, cards.TypeLand)
	if !land && mv <= avail {
		sk.h(fnvAdd(zh, "castable"), 1)
		eachField(cv.Types, func(t string) { sk.h(fnvAdd(fnvAdd(zh, "castable|type|"), t), 1) })
		if instant {
			sk.h(fnvAdd(zh, "castable|instant-speed"), 1)
		}
	}
	if cv.SpellAPI != "" {
		sk.h(fnvAdd(fnvAdd(zh, "api|"), cv.SpellAPI), 1)
	}
	kwh := fnvAdd(zh, "kw|")
	for _, k := range cv.Keywords {
		sk.h(fnvAddLower(kwh, k), 1)
	}
	if hasType(cv, cards.TypeCreature) {
		sk.h(fnvAddInt(fnvAdd(fnvAddInt(fnvAdd(zh, "cr|p"), mzClamp(cv.Power, 9)), "|t"), mzClamp(cv.Toughness, 9)), 1)
	}
}

// mzDiag emits the diagnostic hidden-information tokens.
func mzDiag(v view.View, seat state.PlayerID, fs FeatureSet, diag *Diag, sk *sink) {
	push := sk.str
	var oppAvail int32
	for i := range v.Players {
		if v.Players[i].ID != seat {
			oppAvail += seatAvailable(&v.Players[i])
		}
	}
	for i := range diag.OppHand {
		cv := &diag.OppHand[i]
		push("mz|opphand|"+cv.Name, 1)
		handCard("opphand", cv, oppAvail, sk)
	}
	if fs < FeaturesMZOracle {
		return
	}
	for i, n := range diag.OppLibraryTop {
		push("mz|opplib"+formatInt(int64(i+1))+"|"+n, 1)
		push("mz|opplib|"+n, 1)
	}
	for i, n := range diag.OwnLibraryTop {
		push("mz|ownlib"+formatInt(int64(i+1))+"|"+n, 1)
		push("mz|ownlib|"+n, 1)
	}
}

// cmpFeatureRow orders features by row alone (a stable sort keeps a tie's
// input order).
func cmpFeatureRow(a, b Feature) int { return int(a.Row) - int(b.Row) }
