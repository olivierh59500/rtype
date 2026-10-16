package sim

import "encoding/json"

// cloneOriginalData makes nested declarative source tables independent of their
// caller. The supported tables contain only JSON-compatible scalar and arrays.
func cloneOriginalData[T any](original *T) *T {
	if original == nil {
		return nil
	}
	data, err := json.Marshal(original)
	if err != nil {
		panic("original data table is not serializable")
	}
	var copyOf T
	if err := json.Unmarshal(data, &copyOf); err != nil {
		panic("original data table cannot be copied")
	}
	return &copyOf
}
