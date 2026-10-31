package extern

import (
	"errors"
	"fmt"
	"strings"

	"github.com/adams-shaun/gorge/draft/draftlab"
)

// Endpoint names one external agent's transport. Transport is "exec"
// (Target is the child program's path; a target without '/' is resolved
// with exec.LookPath) or "unix" (Target is the socket path).
type Endpoint struct {
	Transport string
	Target    string
}

// The session-layer errors. Use errors.Is on them: every error this
// package returns wraps exactly one of these (ErrSpec at spec parse time,
// ErrHandshake while dialing, ErrAgentGone for every request that a dead
// or misbehaving session fails, ErrProtocol underneath ErrAgentGone when
// the agent broke the wire).
var (
	// ErrSpec wraps every ParseSpec rejection.
	ErrSpec = errors.New("extern: bad agent spec")
	// ErrHandshake wraps every Dial/DialConn handshake failure; the message
	// names the offending hello field.
	ErrHandshake = errors.New("extern: handshake failed")
	// ErrProtocol marks a wire shape violation. It reaches a caller only
	// wrapped inside ErrAgentGone (a violation kills the session).
	ErrProtocol = errors.New("extern: protocol violation")
	// ErrAgentGone wraps every error a dead or misbehaving session produces,
	// for pending calls, queued calls and later calls alike.
	ErrAgentGone = errors.New("extern: agent is gone")
	// ErrRefused marks an agent's per-item refusal (E5d-4's adapters turn it
	// into a fallback or an error; the session layer delivers it verbatim).
	ErrRefused = errors.New("extern: agent answer refused")
	// ErrRole marks an agent whose hello does not serve the role asked of it
	// (E5d-4's adapters).
	ErrRole = errors.New("extern: agent does not serve this role")
)

// IsSpec reports whether s names an external agent ("ext:..."), the shape
// ParseSpec reads.
func IsSpec(s string) bool { return strings.HasPrefix(s, "ext:") }

// ParseSpec reads "ext:exec:<path>[@<model_version>]" or
// "ext:unix:<path>[@<model_version>]": the part after "ext:" is split on
// the first ':' into transport and target, and the target's trailing
// "@<version>" half follows draftlab.ParseAgent's rule (one '@', non-empty
// halves). The version is the model version recorded on the agent's Ref,
// never part of the transport target. Anything else is an error wrapping
// ErrSpec.
func ParseSpec(spec string) (Endpoint, string, error) {
	rest, ok := strings.CutPrefix(spec, "ext:")
	if !ok {
		return Endpoint{}, "", fmt.Errorf("%w: %q does not start with %q", ErrSpec, spec, "ext:")
	}
	transport, target, ok := strings.Cut(rest, ":")
	if !ok {
		return Endpoint{}, "", fmt.Errorf("%w: %q has no ':' after the transport", ErrSpec, spec)
	}
	switch transport {
	case "exec", "unix":
	default:
		return Endpoint{}, "", fmt.Errorf("%w: transport %q is not exec or unix", ErrSpec, transport)
	}
	ref, err := draftlab.ParseAgent(target)
	if err != nil {
		return Endpoint{}, "", fmt.Errorf("%w: %v", ErrSpec, err)
	}
	return Endpoint{Transport: transport, Target: ref.Name}, ref.Version, nil
}
