package orm

import (
	"database/sql/driver"
	"encoding/json"
	"errors"
)

type Map map[string]interface{}

func (m Map) Value() (driver.Value, error) {
	bytes, err := json.Marshal(m)
	return string(bytes), err
}

func (m *Map) Scan(input interface{}) error {
	switch value := input.(type) {
	case string:
		return json.Unmarshal([]byte(value), m)
	case []byte:
		return json.Unmarshal(value, m)
	default:
		return errors.New("cannot unmarshal value into types.Map")
	}
}
