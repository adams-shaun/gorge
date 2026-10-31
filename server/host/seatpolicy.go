package host

import (
	"fmt"

	"github.com/adams-shaun/gorge/bots"
	"github.com/adams-shaun/gorge/engine/game"
	"github.com/adams-shaun/gorge/engine/seat"
)

// validatedSeatPolicies normalizes and checks the E1-2 TableConfig additions:
// the per-seat bot policies and the owning draft event. It is called from
// validated just before the deck-loading loop, so every refusal happens at
// registration (and at a restart's re-validation) before any seat is built.
func (c TableConfig) validatedSeatPolicies(engine game.Engine) (TableConfig, error) {
	if len(c.SeatBotPolicies) > c.Seats {
		return TableConfig{}, fmt.Errorf("host: table %s: %d seat bot policies for %d seats", c.ID, len(c.SeatBotPolicies), c.Seats)
	}
	if len(c.SeatBotPolicies) > 0 {
		policies := make([]string, len(c.SeatBotPolicies))
		copy(policies, c.SeatBotPolicies)
		for i, name := range policies {
			if name == "" {
				continue
			}
			for _, h := range c.Humans {
				if h == i {
					return TableConfig{}, fmt.Errorf("host: table %s: seat bot policy %q for seat %d: seat %d is human", c.ID, name, i, i)
				}
			}
			policy, err := NormalizeBotPolicy(name)
			if err != nil {
				return TableConfig{}, fmt.Errorf("host: table %s: seat %d: %w", c.ID, i, err)
			}
			policies[i] = policy
			if e, ok := bots.Lookup(policy); ok {
				if e.Search && !engine.Caps().SearchRoots {
					return TableConfig{}, fmt.Errorf("host: table %s: seat %d: bot policy %q searches, and engine %q deals no search roots", c.ID, i, policy, engine.Name())
				}
				if e.ReadsDecklists() {
					return TableConfig{}, fmt.Errorf("host: table %s: seat %d: bot policy %q reads opponents' decklists", c.ID, i, policy)
				}
				if e.NeedsAutoPay && !c.autoPayManaEnabled() {
					return TableConfig{}, fmt.Errorf("host: table %s: seat %d: bot policy %q needs automatic mana payment (auto_mana and bot_auto_pay_mana both on)", c.ID, i, policy)
				}
				if e.MaxSeats > 0 && c.Seats > e.MaxSeats {
					return TableConfig{}, fmt.Errorf("host: table %s: seat %d: bot policy %q seats at most %d, table has %d", c.ID, i, policy, e.MaxSeats, c.Seats)
				}
			}
		}
		c.SeatBotPolicies = policies
	}
	if c.Event != "" && (!c.OnDemand || c.Perpetual) {
		return TableConfig{}, fmt.Errorf("host: table %s: event %q table must be on-demand and not perpetual", c.ID, c.Event)
	}
	if c.Series && c.Event == "" {
		return TableConfig{}, fmt.Errorf("host: table %s: a series table needs an event", c.ID)
	}
	return c, nil
}

// seatBotPolicy is the policy seat i plays: its SeatBotPolicies entry when
// non-empty, else the table's BotPolicy.
func (c TableConfig) seatBotPolicy(i int) string {
	if i < len(c.SeatBotPolicies) && c.SeatBotPolicies[i] != "" {
		return c.SeatBotPolicies[i]
	}
	return c.BotPolicy
}

// seatPolicySeats builds every table bot from the table's per-seat bot
// policies (E1-2): exactly defaultSeatsWithAutoPayMana's loop with
// seatBotPolicy(i) per seat and the table's CommonSense. It panics on a
// bots.New error the way defaultSeatsWithAutoPayMana does — the table was
// validated, so every named policy is registered and buildable.
func seatPolicySeats(c TableConfig, autoPayMana bool, names []string, seed uint64, searchParallelism int, deps bots.Deps, deadlineMS int, configs map[string]map[string]string) []seat.Seat {
	out := make([]seat.Seat, len(names))
	for i := range names {
		bot, err := bots.New(c.seatBotPolicy(i), bots.Options{Seed: seed ^ uint64(i+1), AutoPayMana: autoPayMana, CommonSense: c.BotCommonSense, SearchParallelism: searchParallelism, Deps: deps, DecisionDeadlineMS: deadlineMS, Config: configs[c.seatBotPolicy(i)]})
		if err != nil {
			panic(err) // policy was normalized before the table was registered.
		}
		out[i] = bot
	}
	return out
}
