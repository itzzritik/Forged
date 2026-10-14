package vault

import (
	"encoding/json"
	"reflect"
	"strings"
	"sync"
)

// Extra holds JSON fields this build does not know, so data written by a newer Forged
// survives this build's saves and sync pushes.
type Extra map[string]json.RawMessage

// MergeExtra keeps the first value seen for each field.
func MergeExtra(sets ...Extra) Extra {
	var merged Extra
	for _, set := range sets {
		for name, raw := range set {
			if _, ok := merged[name]; ok {
				continue
			}
			if merged == nil {
				merged = Extra{}
			}
			merged[name] = raw
		}
	}
	return merged
}

var knownFieldNames sync.Map

func fieldNames(t reflect.Type) []string {
	if names, ok := knownFieldNames.Load(t); ok {
		return names.([]string)
	}
	var names []string
	for i := range t.NumField() {
		field := t.Field(i)
		name, _, _ := strings.Cut(field.Tag.Get("json"), ",")
		if name == "-" {
			continue
		}
		if name == "" {
			name = field.Name
		}
		names = append(names, name)
	}
	knownFieldNames.Store(t, names)
	return names
}

func decodeWithExtra[T any](data []byte, v *T, extra *Extra) error {
	if err := json.Unmarshal(data, v); err != nil {
		return err
	}
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(data, &fields); err != nil {
		return err
	}
	for _, known := range fieldNames(reflect.TypeFor[T]()) {
		for name := range fields {
			// encoding/json matches field names case-insensitively.
			if strings.EqualFold(name, known) {
				delete(fields, name)
			}
		}
	}
	*extra = nil
	if len(fields) > 0 {
		*extra = fields
	}
	return nil
}

func encodeWithExtra[T any](v T, extra Extra) ([]byte, error) {
	data, err := json.Marshal(v)
	if err != nil || len(extra) == 0 {
		return data, err
	}
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(data, &fields); err != nil {
		return nil, err
	}
	for name, raw := range extra {
		if _, ok := fields[name]; !ok {
			fields[name] = raw
		}
	}
	return json.Marshal(fields)
}

func (d *VaultData) UnmarshalJSON(data []byte) error {
	type plain VaultData
	return decodeWithExtra(data, (*plain)(d), &d.Extra)
}

func (d VaultData) MarshalJSON() ([]byte, error) {
	type plain VaultData
	return encodeWithExtra(plain(d), d.Extra)
}

func (d *SSHData) UnmarshalJSON(data []byte) error {
	type plain SSHData
	return decodeWithExtra(data, (*plain)(d), &d.Extra)
}

func (d SSHData) MarshalJSON() ([]byte, error) {
	type plain SSHData
	return encodeWithExtra(plain(d), d.Extra)
}

func (r *SSHRoute) UnmarshalJSON(data []byte) error {
	type plain SSHRoute
	return decodeWithExtra(data, (*plain)(r), &r.Extra)
}

func (r SSHRoute) MarshalJSON() ([]byte, error) {
	type plain SSHRoute
	return encodeWithExtra(plain(r), r.Extra)
}

func (t *SSHRouteTombstone) UnmarshalJSON(data []byte) error {
	type plain SSHRouteTombstone
	return decodeWithExtra(data, (*plain)(t), &t.Extra)
}

func (t SSHRouteTombstone) MarshalJSON() ([]byte, error) {
	type plain SSHRouteTombstone
	return encodeWithExtra(plain(t), t.Extra)
}

func (k *Key) UnmarshalJSON(data []byte) error {
	type plain Key
	return decodeWithExtra(data, (*plain)(k), &k.Extra)
}

func (k Key) MarshalJSON() ([]byte, error) {
	type plain Key
	return encodeWithExtra(plain(k), k.Extra)
}

func (m *Metadata) UnmarshalJSON(data []byte) error {
	type plain Metadata
	return decodeWithExtra(data, (*plain)(m), &m.Extra)
}

func (m Metadata) MarshalJSON() ([]byte, error) {
	type plain Metadata
	return encodeWithExtra(plain(m), m.Extra)
}

func (t *Tombstone) UnmarshalJSON(data []byte) error {
	type plain Tombstone
	return decodeWithExtra(data, (*plain)(t), &t.Extra)
}

func (t Tombstone) MarshalJSON() ([]byte, error) {
	type plain Tombstone
	return encodeWithExtra(plain(t), t.Extra)
}
