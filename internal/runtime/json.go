package runtime

import (
	"encoding/json"
	"io"
)

// DecodeJSON reads exactly one JSON value. HTTP handlers use this helper so a
// valid object followed by another value cannot be silently accepted.
func DecodeJSON(reader io.Reader, destination any) error {
	decoder := json.NewDecoder(reader)
	if err := decoder.Decode(destination); err != nil {
		return err
	}
	var trailing any
	if err := decoder.Decode(&trailing); err != io.EOF {
		if err == nil {
			return io.ErrUnexpectedEOF
		}
		return err
	}
	return nil
}
