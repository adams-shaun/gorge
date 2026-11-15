package deck

import (
	"math/bits"

	"github.com/adams-shaun/gorge/engine/cards"
)

// HasImpliedDraftPartner reports whether c may be treated as having partner
// under the Commander-draft rule Mystery Booster Commander drafts play by: a
// commander-eligible card whose colour identity has at most one colour and
// that carries no partner-family ability of its own (Partner, Partner with,
// partner—[text], Doctor's companion, Choose a Background) may be treated as
// having plain partner.
func HasImpliedDraftPartner(c *cards.Card) bool {
	if c == nil || len(c.Faces) == 0 || !IsCommanderEligible(c) {
		return false
	}
	if bits.OnesCount8(c.ColourIdentity()) > 1 {
		return false
	}
	return partnerHead(c) == "" && !hasChooseABackground(c) && !hasDoctorCompanion(c)
}

// hasPlainPartner reports whether c's front face carries plain partner
// (K:Partner with no partner—[text]).
func hasPlainPartner(c *cards.Card) bool {
	for _, t := range partnerTexts(c) {
		if t == "" {
			return true
		}
	}
	return false
}

// IsDraftCommanderPair reports whether a and b may be a Commander-draft
// seat's two commanders: any legal pair (IsCommanderPair), or two distinct
// commander-eligible cards each of which has plain partner or may be
// treated as having it (HasImpliedDraftPartner). A drafted pool may hold two
// copies of one card; they are never a pair.
func IsDraftCommanderPair(a, b *cards.Card) bool {
	if a == nil || b == nil || len(a.Faces) == 0 || len(b.Faces) == 0 {
		return false
	}
	if cards.NormalizeName(a.Faces[0].Name) == cards.NormalizeName(b.Faces[0].Name) {
		return false
	}
	if IsCommanderPair(a, b) {
		return true
	}
	return IsDraftPairHalf(a) && IsDraftPairHalf(b)
}

// IsDraftPairHalf reports whether c may be one half of a Commander-draft
// pair under the draft rule alone: a commander-eligible card with plain
// partner, or one that may be treated as having it (HasImpliedDraftPartner).
// Partner-with, partner—[text], Doctor's-companion and Background pairs are
// IsCommanderPair's, and pair only with their own counterpart.
func IsDraftPairHalf(c *cards.Card) bool {
	if c == nil || len(c.Faces) == 0 {
		return false
	}
	return IsCommanderEligible(c) && (hasPlainPartner(c) || HasImpliedDraftPartner(c))
}
