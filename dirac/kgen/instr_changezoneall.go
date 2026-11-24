package kgen

import (
	"fmt"
	"strconv"

	"github.com/adams-shaun/gorge/dirac/kop"
)

// The kop forms of the LIR ops for mass zone moves (instr.go).
func init() {
	instrOp("ChangeZoneAll", (*instrCtx).opChangeZoneAll)
}

func (ic *instrCtx) opChangeZoneAll() error {
	ki := &ic.ki
	in := &ic.in
	cb := ic.cb
	fl := ic.fl
	from := zoneNames[fl["from"]]
	if fl["from"] == "Graveyard,Exile" {
		// graveexile: every remembered match in each player's Graveyard then
		// Exile (gorge changeZoneAllPlayers over both zones).
		if err := ic.objFilter(kop.ZoneNone); err != nil {
			return err
		}
		ki.Op = kop.OpChangeZoneAllEx
		ki.C = int16(int(zoneMap[in.C]) | (ic.moveFlags()|kop.MvGraveExile)<<3)
		return nil
	}
	if b, ok := czallTwoZoneB(fl["from"]); ok {
		// two origins swept zone-major over every seat; the second zone
		// (or a Cz* selector) rides in B, ZoneNone lets the owner role build
		if err := ic.objFilter(kop.ZoneNone); err != nil {
			return err
		}
		ki.Op = kop.OpChangeZoneAllEx
		ki.C = int16(int(zoneMap[in.C]) | (ic.moveFlags()|kop.MvGraveExile)<<3)
		if _, ok := fl["ShuffleEach"]; ok {
			b |= kop.CzShuffleEach // the kernel shuffles both libraries after the sweep
		}
		ki.B = kop.Const(b)
		return nil
	}
	if err := ic.objFilter(zoneMap[from]); err != nil {
		return err
	}
	ki.Op, ki.C = kop.OpChangeZoneAll, int16(zoneMap[in.C])
	if _, ok := fl["chosenOrder"]; ok {
		ki.C |= kop.CzChosenOrder
	}
	_, untilLeaves := fl["UntilLeaves"]
	if m := ic.moveFlags(); m != 0 || in.Obj.Kind != rNone || untilLeaves {
		ki.Op = kop.OpChangeZoneAllEx
		ki.C = int16(int(zoneMap[in.C]) | m<<3)
	}
	if fl["counters"] != "" || fl["static"] != "" || fl["atEOT"] != "" {
		// WithCountersType$ / StaticEffect$ on a sweep onto the
		// battlefield: A = constant count | counter kind<<8, B = override
		// index+1 (the index is pool-wide, so B gets all 16 bits); the
		// kernel applies both per entrant (kop.MvEntryRiders)
		if zoneMap[in.C] != kop.ZoneBattlefield {
			return fmt.Errorf("ChangeZoneAll entry riders to zone %d", in.C)
		}
		ki.Op = kop.OpChangeZoneAllEx
		ki.C = int16(int(zoneMap[in.C]) | ic.moveFlags()<<3)
		ki.A, ki.B = kop.Const(0), kop.Const(0)
		if k, ok := fl["counters"]; ok {
			ck, err := counterKind(k)
			if err != nil {
				return err
			}
			if !in.A.isConst() || in.A.N < 1 || in.A.N > 255 {
				return fmt.Errorf("ChangeZoneAll counter count is not a constant 1..255")
			}
			ki.A = kop.Const(int(ck)<<8 | in.A.N)
		}
		if idx, ok := fl["static"]; ok {
			n, err := strconv.Atoi(idx)
			if err != nil {
				return fmt.Errorf("ChangeZoneAll static=%q", idx)
			}
			ov, err := cb.override(cb.c.overrides[n])
			if err != nil {
				return err
			}
			if ov+1 > 0xffff {
				return fmt.Errorf("override index %d above 65534", ov)
			}
			ki.B = kop.Const(ov + 1)
		}
		if a, ok := fl["atEOT"]; ok {
			// one next-end-step promise per entrant,
			// kop.EntryAtEOT in A's high bit (kind <= kop.EntryKindMask), the act in
			// kop.EntryAtEOTExile
			act, err := strconv.Atoi(a)
			if err != nil || act < 0 || act > 1 {
				return fmt.Errorf("ChangeZoneAll atEOT=%q", a)
			}
			if ck := int(uint16(ki.A.N)) >> 8; ck > kop.EntryKindMask {
				return fmt.Errorf("ChangeZoneAll counter kind %d collides with the AtEOT bits", ck)
			}
			ki.A.N |= entryAtEOTBits(act)
		}
		// MvEntryRiders is bit 12, so its shifted C image is the sign bit:
		// shift a variable (the constant 1<<15 does not fit int16).
		er := int16(kop.MvEntryRiders)
		ki.C |= er << 3
	}
	if _, forget := fl["ForgetOther"]; forget || untilLeaves {
		if err := czallBuild3(ki, zoneMap[in.C], forget, untilLeaves); err != nil { // lower_changezoneall.go
			return err
		}
	}
	return nil
}
