package protected

import (
	"encoding/json"
	models "thanawy-backend/internal/domain/common"
)

// parseStringArray converts a raw interface{} value (usually from JSON) to a PGStringArray pointer.
func parseStringArray(val interface{}) (*models.PGStringArray, error) {
	if val == nil {
		return nil, nil
	}
	b, err := json.Marshal(val)
	if err != nil {
		return nil, err
	}
	var sa models.PGStringArray
	if err := json.Unmarshal(b, &sa); err != nil {
		return nil, err
	}
	return &sa, nil
}
