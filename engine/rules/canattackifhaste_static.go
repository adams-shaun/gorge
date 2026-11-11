package rules

import "github.com/adams-shaun/gorge/engine/effects"

// stat:CanAttackIfHaste is the printed S:Mode$ CanAttackIfHaste static (Instill
// Energy, Chaos Lord, Frenzied Saddlebrute): a summoning-sick creature may be
// declared as an attacker as though it had haste. The reader is
// engine/rules/combat's CanAttackIfHastePair, consulted from CanAttack and
// CanAttackPairAt; this init only registers the support token.
func init() { effects.RegisterNonAPI("stat:CanAttackIfHaste") }
