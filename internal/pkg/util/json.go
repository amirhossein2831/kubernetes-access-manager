package util

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
)

const MaxInputBytes = 8 * 1024 * 1024

func DecodeJSON(b []byte, target any) error {
	if len(b) > MaxInputBytes {
		return errors.New("startup input exceeds 8 MiB")
	}
	dec := json.NewDecoder(bytes.NewReader(b))
	if err := uniqueJSON(dec); err != nil {
		return err
	}
	if _, err := dec.Token(); err != io.EOF {
		return errors.New("trailing JSON data")
	}
	dec = json.NewDecoder(bytes.NewReader(b))
	dec.DisallowUnknownFields()
	if err := dec.Decode(target); err != nil {
		return fmt.Errorf("invalid startup JSON: %w", err)
	}
	return nil
}

func uniqueJSON(d *json.Decoder) error {
	token, err := d.Token()
	if err != nil {
		return err
	}
	delim, ok := token.(json.Delim)
	if !ok {
		return nil
	}
	switch delim {
	case '{':
		seen := map[string]bool{}
		for d.More() {
			key, err := d.Token()
			if err != nil {
				return err
			}
			k, ok := key.(string)
			if !ok {
				return errors.New("invalid object key")
			}
			if seen[k] {
				return errors.New("duplicate JSON key")
			}
			seen[k] = true
			if err := uniqueJSON(d); err != nil {
				return err
			}
		}
	case '[':
		for d.More() {
			if err := uniqueJSON(d); err != nil {
				return err
			}
		}
	default:
		return errors.New("invalid JSON delimiter")
	}
	_, err = d.Token()
	return err
}
