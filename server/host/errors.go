package host

import (
	"errors"
	"fmt"
)

// ErrEngineUnsupported refuses a feature that reads gorge's rules engine, on a
// table running another engine.
var ErrEngineUnsupported = errors.New("host: unsupported engine")

func engineRefusal(feature, engine string) error {
	return fmt.Errorf("%w: %s needs gorge, this table runs %q", ErrEngineUnsupported, feature, engine)
}
