package host

import (
	"fmt"

	"github.com/adams-shaun/gorge/seat"
)

const (
	// BotPolicy is the stable deterministic hosted bot policy.
	BotPolicy = "bot"
	// LethalPressurePolicy is the measured opt-in hosted experiment.
	LethalPressurePolicy = "lethal-pressure"
)

// NormalizeBotPolicy returns a hosted policy's stable name. An omitted name
// deliberately selects the production bot so tables saved before policy
// selection existed retain their behavior. The hosted vocabulary is closed:
// diagnostic policies such as legacy cannot silently become opponents.
func NormalizeBotPolicy(name string) (string, error) {
	if name == "" {
		return BotPolicy, nil
	}
	switch name {
	case BotPolicy, LethalPressurePolicy:
		return name, nil
	default:
		return "", fmt.Errorf("host: unknown bot policy %q (known: bot, lethal-pressure)", name)
	}
}

// NewBotPolicySeat builds a fresh deterministic seat for a supported hosted
// policy. The caller owns the per-seat seed derivation; the factory never
// reaches ambient randomness or substitutes a policy on an unknown name.
func NewBotPolicySeat(name string, seed uint64) (seat.Seat, error) {
	name, err := NormalizeBotPolicy(name)
	if err != nil {
		return nil, err
	}
	if name == LethalPressurePolicy {
		return seat.NewLethalPressureBot(seed), nil
	}
	return seat.NewBot(seed), nil
}
