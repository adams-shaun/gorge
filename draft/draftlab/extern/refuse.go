package extern

import "fmt"

// refused answers one refused item per Options: with StrictRefusals it
// returns an error wrapping ErrRefused (naming the ref and the reason) and
// runs no fallback; otherwise it counts Stats.Refused, logs one line and
// runs the caller's fallback answer. kind is the item kind ("pick",
// "build" or "decide"); reason is the refusal's cause, already phrased.
func (a *Agent) refused(kind, reason string, fallback func()) error {
	if a.opts.StrictRefusals {
		return fmt.Errorf("%w: %s %s: %s", ErrRefused, a.Ref(), kind, reason)
	}
	a.statsRefused.Add(1)
	a.logf("extern: %s %s refused (%s): answered by fallback", a.Ref(), kind, reason)
	fallback()
	return nil
}

// failed answers one session failure per Options.OnFailure (E5d-2's rule):
// FailError (the default) returns the error wrapping ErrAgentGone;
// FailFallback counts Stats.Fallbacks, logs one line and runs the caller's
// fallback answer.
func (a *Agent) failed(err error, kind string, fallback func()) error {
	if a.opts.OnFailure == FailFallback {
		a.statsFallbacks.Add(1)
		a.logf("extern: %s %s session failed (%s): answered by fallback", a.Ref(), kind, err)
		fallback()
		return nil
	}
	return err
}

// logf writes one line through Options.Log when set.
func (a *Agent) logf(format string, args ...any) {
	if a.opts.Log != nil {
		a.opts.Log(fmt.Sprintf(format, args...))
	}
}

// dead reports a session that is dead or closing.
func (a *Agent) dead() bool { return a.sess.deadFlag.Load() || a.sess.isClosing() }
