package runtime

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
)

type runSpecWithoutMethods RunSpec

type runSpecWire struct {
	runSpecWithoutMethods
	Profile    string  `json:"profile,omitempty"`
	RunIDAlias *string `json:"run_id,omitempty"`
}

// MarshalJSON preserves the long-standing legacy execution wire while adding
// the profile required to validate the closed subscription authority. A
// control-plane profile never becomes a new legacy/native execution input.
func (s RunSpec) MarshalJSON() ([]byte, error) {
	profile := ""
	if s.Authority != "" {
		profile = s.Profile
	}
	return json.Marshal(runSpecWire{
		runSpecWithoutMethods: runSpecWithoutMethods(s),
		Profile:               profile,
	})
}

// UnmarshalJSON rejects a profile on empty legacy authority instead of
// silently accepting a wire shape that this version would never emit.
func (s *RunSpec) UnmarshalJSON(data []byte) error {
	if s == nil {
		return fmt.Errorf("runtime: nil run spec")
	}
	var wire runSpecWire
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&wire); err != nil {
		return err
	}
	var trailing any
	if err := decoder.Decode(&trailing); !errors.Is(err, io.EOF) {
		if err == nil {
			return fmt.Errorf("runtime: trailing run spec value")
		}
		return err
	}
	decoded := RunSpec(wire.runSpecWithoutMethods)
	if wire.RunIDAlias != nil {
		if decoded.RunID != "" && decoded.RunID != *wire.RunIDAlias {
			return fmt.Errorf("runtime: conflicting run id fields")
		}
		decoded.RunID = *wire.RunIDAlias
	}
	if decoded.Authority == "" && wire.Profile != "" {
		return fmt.Errorf("runtime: legacy wire contains profile")
	}
	decoded.Profile = wire.Profile
	*s = decoded
	return nil
}
