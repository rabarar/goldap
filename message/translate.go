package message

import (
	"errors"
	"fmt"
	"reflect"
	"strings"
)

const (
	TransVTInt    TransValType = 1
	TransVTString TransValType = 2
	TransVTFloat  TransValType = 3
)

var (
	translatorTypeToString map[TransValType]string = map[TransValType]string{
		TransVTInt:    "int",
		TransVTString: "string",
		TransVTFloat:  "float",
	}
)

type TransValType int

type TranslatorElem struct {
	Name    string       `json:"name"`
	Type    TransValType `json:"type"`
	IsArray bool         `json:"isArray"`
}
type TranslatorMap map[string]TranslatorElem

type Translator struct {
	Table TranslatorMap
}

func (t *Translator) Replace(key string) (string, *TranslatorElem) {
	xlat, ok := t.Table[key]
	if ok {
		return `"` + xlat.Name + `"`, &xlat
	}
	return `"` + key + `"`, nil
}

func (t TransValType) MarshalJSON() ([]byte, error) {
	ts, ok := translatorTypeToString[t]
	if !ok {
		return []byte{}, errors.New("No Such Translation Type")
	}
	return []byte(fmt.Sprintf(`"%s"`, ts)), nil
}

func (t *TransValType) UnmarshalJSON(val []byte) error {

	switch strings.ToLower(strings.Trim(string(val), `"`)) {
	case "int":
		*t = TransVTInt
	case "string":
		*t = TransVTString
	case "float":
		*t = TransVTFloat
	default:
		return errors.New(fmt.Sprintf("Invalid Translation Type: [%s]", string(val)))
	}

	return nil
}

func (s *SearchRequest) FilterMap(lookup Translator, wrap bool) string {
	str, _ := s.decompileFilterMap(s.Filter(), lookup)
	if wrap {
		return "{ " + str + " }"
	} else {
		return str
	}
}

func (s *SearchRequest) decompileFilterMap(packet Filter, lookup Translator) (ret string, err error) {

	defer func() {
		if r := recover(); r != nil {
			err = errors.New("error decompiling filter")
		}
	}()

	err = nil
	childStr := ""

	switch f := packet.(type) {
	case FilterAnd:
		var notFirst bool = false
		for _, child := range f {

			if notFirst {
				ret += ", "
			}

			childStr, err = s.decompileFilterMap(child, lookup)
			if err != nil {
				return
			}
			notFirst = true
			ret += childStr
		}
	case FilterOr:
		ret += `"$or": [`
		var notFirst bool = false
		for _, child := range f {
			if notFirst {
				ret += ", "
			}
			childStr, err = s.decompileFilterMap(child, lookup)
			if err != nil {
				return
			}
			ret += "{"
			ret += childStr
			ret += "}"
			notFirst = true
		}
		ret += "]"
	case FilterNot:
		// { $not: { $gt: 1.99 } } TODO - i don't think this will work... need to test and see how it propogates over and/or
		childStr, err = s.decompileFilterMap(f.Filter, lookup)
		if err != nil {
			return
		}
		ret += `{ "$not:" `
		ret += childStr
		ret += "}"

	case FilterSubstrings:
		ret += `"` + string(f.Type_()) + `"`
		ret += ":"
		for _, fs := range f.Substrings() {
			switch fsv := fs.(type) {
			case SubstringInitial:
				ret += fmt.Sprintf(`"/^%s/"`, string(fsv)+"*")
			case SubstringAny:
				ret += fmt.Sprintf(`"/%s/"`, "*"+string(fsv)+"*")
			case SubstringFinal:
				ret += fmt.Sprintf(`"/$s/"`, "*"+string(fsv))
			}
		}
	case FilterEqualityMatch:
		new, xl := lookup.Replace(string(f.AttributeDesc()))
		ret += new
		ret += ":"
		if xl != nil {
			switch xl.Type {
			case TransVTInt, TransVTFloat:
				if xl.IsArray {
					ret += fmt.Sprintf(`{"$in": [%s]}`, string(f.AssertionValue()))
				} else {
					ret += fmt.Sprintf(`{"$eq": %s}"`, string(f.AssertionValue()))
				}
			case TransVTString:
				if xl.IsArray {
					ret += fmt.Sprintf(`{"$in": ["%s"]}`, string(f.AssertionValue()))
				} else {
					ret += fmt.Sprintf(`{"$eq": "%s"}`, string(f.AssertionValue()))
				}
			}
		} else {
			ret += fmt.Sprintf(`{"$eq": "%s"}`, string(f.AssertionValue()))
		}
	case FilterGreaterOrEqual:
		new, _ := lookup.Replace(string(f.AttributeDesc()))
		ret += new
		ret += ":"
		ret += fmt.Sprintf(`{"$gte": %s}`, string(f.AssertionValue()))
	case FilterLessOrEqual:
		new, _ := lookup.Replace(string(f.AttributeDesc()))
		ret += new
		ret += ":"
		ret += fmt.Sprintf(`{"$lte": %s}`, string(f.AssertionValue()))
	case FilterPresent:
		// if 0 == len(packet.Children) {
		// 	ret += ber.DecodeString(packet.Data.Bytes())
		// } else {
		// 	ret += ber.DecodeString(packet.Children[0].Data.Bytes())
		// }
		ret += `"` + string(f) + `"`
		ret += ":"
		ret += `{"$exists": true}`
	case FilterApproxMatch:
		new, _ := lookup.Replace(string(f.AttributeDesc()))
		ret += new
		ret += ":"
		ret += fmt.Sprintf(`"/%s/"`, string(f.AssertionValue()))
	}

	return ret, nil
}

func jsonTag(v interface{}, name string) string {
	return fieldTag(v, "json", name)
}

func fieldTag(v interface{}, tag, name string) string {

	newValType := reflect.TypeOf(v)

	sf, ok := newValType.FieldByName(name)
	if ok {
		return strings.Split(sf.Tag.Get(tag), ",")[0]
	}
	return ""
}
