package dto

import "encoding/json"

// Optional distinguishes three JSON states for PATCH-style requests:
//
//	field absent      -> Defined == false              (do not change)
//	field is null     -> Defined == true, Value == nil (clear the column)
//	field has a value -> Defined == true, Value != nil  (set the column)
//
// Plain *T cannot tell "absent" from "null" because encoding/json decodes
// both to nil.
type Optional[T any] struct {
	Defined bool
	Value   *T
}

// UnmarshalJSON is only invoked by encoding/json when the key is present in
// the payload, so it is enough to mark the field as Defined here.
func (o *Optional[T]) UnmarshalJSON(data []byte) error {
	o.Defined = true
	if string(data) == "null" {
		o.Value = nil
		return nil
	}
	return json.Unmarshal(data, &o.Value)
}
