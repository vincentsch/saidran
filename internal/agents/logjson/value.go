package logjson

import "encoding/json"

type Object map[string]json.RawMessage

func (o Object) String(k string) string { var v string; json.Unmarshal(o[k], &v); return v }
func (o Object) Bool(k string) bool     { var v bool; json.Unmarshal(o[k], &v); return v }
func (o Object) Object(k string) Object { var v Object; json.Unmarshal(o[k], &v); return v }
func (o Object) List(k string) []Object { var v []Object; json.Unmarshal(o[k], &v); return v }
func (o Object) Int(k string) *int      { var v *int; json.Unmarshal(o[k], &v); return v }
func (o Object) Text(k string) string {
	if s := o.String(k); s != "" {
		return s
	}
	s := ""
	for _, b := range o.List(k) {
		if t := b.String("type"); t == "text" || t == "Text" || t == "input_text" || t == "output_text" {
			if s != "" {
				s += "\n"
			}
			s += b.String("text")
		}
	}
	return s
}
