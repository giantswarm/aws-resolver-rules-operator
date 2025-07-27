package v1alpha1

import (
	"encoding/json"
	"fmt"
	"slices"
	"time"

	"github.com/samber/lo"
)

const Never = "Never"

// NillableDuration is a wrapper around time.Duration which supports correct
// marshaling to YAML and JSON. It uses the value "Never" to signify
// that the duration is disabled and sets the inner duration as nil
type NillableDuration struct {
	*time.Duration

	// Raw is used to ensure we remarshal the NillableDuration in the same format it was specified.
	// This ensures tools like Flux and ArgoCD don't mistakenly detect drift due to our conversion webhooks.
	Raw []byte `hash:"ignore"`
}

func MustParseNillableDuration(val string) NillableDuration {
	nd := NillableDuration{}
	// Use %q instead of %s to ensure that we unmarshal the value as a string and not an int
	lo.Must0(json.Unmarshal([]byte(fmt.Sprintf("%q", val)), &nd))
	return nd
}

// UnmarshalJSON implements the json.Unmarshaller interface.
func (d *NillableDuration) UnmarshalJSON(b []byte) error {
	var str string
	err := json.Unmarshal(b, &str)
	if err != nil {
		return err
	}
	if str == Never {
		return nil
	}
	pd, err := time.ParseDuration(str)
	if err != nil {
		return err
	}
	d.Raw = slices.Clone(b)
	d.Duration = &pd
	return nil
}

// MarshalJSON implements the json.Marshaler interface.
func (d NillableDuration) MarshalJSON() ([]byte, error) {
	if d.Raw != nil {
		return d.Raw, nil
	}
	if d.Duration != nil {
		return json.Marshal(d.Duration.String())
	}
	return json.Marshal(Never)
}

// ToUnstructured implements the value.UnstructuredConverter interface.
func (d NillableDuration) ToUnstructured() interface{} {
	if d.Raw != nil {
		// Decode the JSON bytes to get the actual string value
		var str string
		if err := json.Unmarshal(d.Raw, &str); err == nil {
			return str
		}
		// Fallback to string conversion if unmarshal fails
		if d.Duration != nil {
			return d.Duration.String()
		}
		return Never
	}
	if d.Duration != nil {
		return d.Duration.String()
	}
	return Never
}
