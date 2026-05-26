package ast

import (
	"fmt"
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"
	"encoding/binary"
)

/*******************************************************************************
 * Value interface
 ******************************************************************************/ 

type valueI interface {
	expression
	marshaler
}

func valueType(tdef typedef) bool {
	switch td := tdef.getType().(type) {
        case *boolType:
        	return true
        case *intType:
        	return true
        case *floatType:
        	return true
        case *stringType:
        	return true
        case *listType:
        	return valueType(td.tdef)
       	case *recordType:
       		for _, t := range td.tdefs {
       			if !valueType(t) { return false }
       		}
       		return true
        default:
        	return false
    }
}

/*******************************************************************************
 * Marshaler
 ******************************************************************************/

type marshaler interface {
	marshal() string
}

func (it *intExpr) marshal() string {
	return it.String()
}

func (ft *floatExpr) marshal() string {
	return ft.String()
}

func (t *trueExpr) marshal() string {
	return t.String()
}

func (f *falseExpr) marshal() string {
	return f.String()
}

func (s *stringExpr) marshal() string {
	return s.String()
}

func (le *listExpr) marshal() string {
	var s string
	for i, e := range le.expressions {
		v, ok := e.(marshaler)
		if !ok { continue }
		if i != 0 {
			s += ", "
		}
		s += v.marshal()
	}
	return "[" + s + "]"
}

func (re *recordExpr) marshal() string {
	var s string
	for i, e := range re.expressions {
		v, ok := e.(marshaler)
		if !ok { continue }
		if i != 0 {
			s += ", "
		}
		s += "\"" + re.labels[i] + "\"" + ": " + v.marshal()
	}
	return "{" + s + "}"
}

/*******************************************************************************
 * unmarshaler
 ******************************************************************************/

type unmarshaler interface {
	unmarshal(string) (expression, string, error)
}

func unmarshal(tdef typedef, s string) (expression, error) {
	um, ok := tdef.getType().(unmarshaler)
	if !ok {
		return tdef.defaultValue(), fmt.Errorf("unrecognisable type: %q", tdef.String())
	}
	expr, rest, err := um.unmarshal(s)
	if err != nil {
		return expr, err
	}
	if rest != "" {
		return expr, fmt.Errorf("string not consumed; remaining: %q", preview(rest))
	}
	return expr, err
}

func (it *intType) unmarshal(s string) (expression, string, error) {
	s0 := s
    s = strings.TrimFunc(s, unicode.IsSpace)
    if len(s) == 0 {
        return it.defaultValue(), s0, fmt.Errorf("empty input")
    }

	tok, rest, ok := readNumberToken(s, false)
    if !ok {
        return it.defaultValue(), s0, fmt.Errorf("expected int at %q", preview(s))
    }
    if _, err := strconv.ParseInt(tok, 10, 64); err != nil {
        return it.defaultValue(), s0, fmt.Errorf("invalid int %q: %v", tok, err)
    }
    return newIntExpr(tok, it.line), rest, nil
}

func (ft *floatType) unmarshal(s string) (expression, string, error) {
	s0 := s
    s = strings.TrimFunc(s, unicode.IsSpace)
    if len(s) == 0 {
        return ft.defaultValue(), s0, fmt.Errorf("empty input")
    }

	tok, rest, ok := readNumberToken(s, true)
    if !ok {
        return ft.defaultValue(), s0, fmt.Errorf("expected float at %q", preview(s))
    }
    if _, err := strconv.ParseFloat(tok, 64); err != nil {
        return ft.defaultValue(), s0, fmt.Errorf("invalid float %q: %v", tok, err)
    }
    return newFloatExpr(tok, ft.line), rest, nil
}

func (bt *boolType) unmarshal(s string) (expression, string, error) {
	s0 := s
    s = strings.TrimFunc(s, unicode.IsSpace)
    if len(s) == 0 {
        return bt.defaultValue(), s0, fmt.Errorf("empty input")
    }

	if strings.HasPrefix(s, "true") && boundary(s, 4) {
        return newTrueExpr(bt.line), strings.TrimLeftFunc(s[4:], unicode.IsSpace), nil
    }
    if strings.HasPrefix(s, "false") && boundary(s, 5) {
        return newFalseExpr(bt.line), strings.TrimLeftFunc(s[5:], unicode.IsSpace), nil
    }
    return bt.defaultValue(), s0, fmt.Errorf("expected bool at %q", preview(s))
}

func (st *stringType) unmarshal(s string) (expression, string, error) {
	s0 := s
    s = strings.TrimFunc(s, unicode.IsSpace)
    if len(s) == 0 {
        return st.defaultValue(), s0, fmt.Errorf("empty input")
    }

	tok, rest, ok := readStringToken(s)
    if !ok {
        return st.defaultValue(), s0, fmt.Errorf("expected string at %q", preview(s))
    }
    return newStringExpr(tok, st.line), rest, nil
} 

func (lt *listType) unmarshal(s string) (expression, string, error) {
	s0 := s
    s = strings.TrimFunc(s, unicode.IsSpace)
    if len(s) == 0 {
        return lt.defaultValue(), s0, fmt.Errorf("empty input")
    }

	if s[0] != '[' {
		return lt.defaultValue(), s0, fmt.Errorf("expected list at %q", preview(s))
	}
	s = strings.TrimFunc(s[1:], unicode.IsSpace)

	if len(s) > 0 && s[0] == ']' {
		return newListExpr(nil, lt.line), strings.TrimFunc(s[1:], unicode.IsSpace), nil
	}

	var expressions []expression
	for {
		if len(s) == 0 {
			return lt.defaultValue(), s, fmt.Errorf("unterminated list")
		}

		um, ok := lt.tdef.(unmarshaler)
		if !ok {
			return lt.defaultValue(), s0, fmt.Errorf("unrecognisable at %q", preview(s))
		} 
		expr, rest, err := um.unmarshal(s)
		if err != nil {
			return lt.defaultValue(), rest, err
		}
		expressions = append(expressions, expr)

		s = strings.TrimFunc(rest, unicode.IsSpace)
		if len(s) == 0 {
			return lt.defaultValue(), s, fmt.Errorf("unterminated list")
		}

		switch s[0] {
			case ',':
				s = strings.TrimFunc(s[1:], unicode.IsSpace)
			case ']':
				return newListExpr(expressions, lt.line), strings.TrimFunc(s[1:], unicode.IsSpace), nil
			default:
				return lt.defaultValue(), s, fmt.Errorf("expected ',' or ']' at %q", preview(s))
		}
	}
}

func (rt *recordType) unmarshal(s string) (expression, string, error) {
	s0 := s
    s = strings.TrimFunc(s, unicode.IsSpace)
    if len(s) == 0 {
        return rt.defaultValue(), s0, fmt.Errorf("empty input")
    }

	if s[0] != '{' {
		return rt.defaultValue(), s0, fmt.Errorf("expected record at %q", preview(s))
	}

	s = strings.TrimFunc(s[1:], unicode.IsSpace)

	var labels []string
	var expressions []expression

	if len(s) > 0 && s[0] == '}' {
		return newRecordExpr(labels, expressions, rt.line), strings.TrimFunc(s[1:], unicode.IsSpace), nil
	}

	for {
	    if len(s) == 0 {
			return rt.defaultValue(), s, fmt.Errorf("unterminated record")
		}

		key, rest, ok := readStringToken(s)
		if !ok {
			return rt.defaultValue(), s0, fmt.Errorf("expected string key at %q", preview(s))
		}

		s = strings.TrimFunc(rest, unicode.IsSpace)

		if len(s) == 0 || s[0] != ':' {
			return rt.defaultValue(), s0, fmt.Errorf("expected ':' after key at %q", preview(s))
		}

		fieldT, ok := rt.tdefMap[key]
		if !ok {
			return rt.defaultValue(), s0, fmt.Errorf("unexpected label %q", key)
		}

		um, ok := fieldT.(unmarshaler)
		if !ok {
			return rt.defaultValue(), s0, fmt.Errorf("unrecognisable at %q", preview(s))
		}

		s = strings.TrimFunc(s[1:], unicode.IsSpace)

		val, rest, err := um.unmarshal(s)
		if err != nil {
			return rt.defaultValue(), rest, err
		}

		labels = append(labels, key)
		expressions = append(expressions, val)
		s = strings.TrimFunc(rest, unicode.IsSpace)

		if len(s) == 0 {
			return rt.defaultValue(), s, fmt.Errorf("unterminated record")
		}

        switch s[0] {
        	case ',':
				s = strings.TrimFunc(s[1:], unicode.IsSpace)
			case '}':
				if len(labels) < len(rt.labels) {
					return rt.defaultValue(), s, fmt.Errorf("recognised record: missing labels.")
				}
				return newRecordExpr(labels, expressions, rt.line), strings.TrimFunc(s[1:], unicode.IsSpace), nil
			default:
				return rt.defaultValue(), s, fmt.Errorf("expected ',' or '}' at %q", preview(s))
		}
    }
}

/* ---------------- unmarshal helpers ---------------- */

func preview(s string) string {
    if len(s) > 32 { s = s[:32] + "…" }
    return s
}

// boundary: ensure the next rune ends an identifier (for "true"/"false")
func boundary(s string, k int) bool {
    if k >= len(s) { return true }
    r, _ := utf8.DecodeRuneInString(s[k:])
    return !(unicode.IsLetter(r) || unicode.IsDigit(r) || r == '_')
}

// readNumberToken scans an int or float (with optional sign and exponent).
func readNumberToken(s string, allowFloat bool) (tok, rest string, ok bool) {
    i := 0
    if s[i] == '+' || s[i] == '-' {
        i++
        if i >= len(s) { return "", s, false }
    }
    digits := 0
    for i < len(s) && isDigit(s[i]) { i++; digits++ }

    if allowFloat && i < len(s) && s[i] == '.' {
        i++
        for i < len(s) && isDigit(s[i]) { i++ }
    } else if digits == 0 {
        return "", s, false
    }

    // optional exponent
    if allowFloat && i < len(s) && (s[i] == 'e' || s[i] == 'E') {
        j := i + 1
        if j < len(s) && (s[j] == '+' || s[j] == '-') { j++ }
        k := j
        for k < len(s) && isDigit(s[k]) { k++ }
        if k == j { // no digits after e/E
            // back off: ignore exponent
        } else {
            i = k
        }
    }
    tok = s[:i]
    rest = strings.TrimLeftFunc(s[i:], unicode.IsSpace)
    return tok, rest, true
}

func isDigit(b byte) bool { return b >= '0' && b <= '9' }

func readStringToken(s string) (tok, rest string, ok bool) {
    if len(s) == 0 {
        return "", s, false
    }

    if (s[0] != '"' && s[0] != '\'') {
    	return s, "", true
    }

    q := s[0]
    i := 1
    for i < len(s) {
        c := s[i]
        if c == '\\' {
            i += 2
            continue
        }
        if c == q {
            tok = s[1:i]
            rest = strings.TrimLeftFunc(s[i+1:], unicode.IsSpace)
            return tok, rest, true
        }
        i++
    }
    // unterminated string: consume whole input as token
    return s, "", true
}


/*******************************************************************************
 * byte unmarshaler
 ******************************************************************************/

type byteUnmarshaler interface {
	byteUnmarshal(string) ([]byte, string, error)
}

func byteUnmarshal(tdef typedef, s string) ([]byte, error) {
	um, ok := tdef.getType().(byteUnmarshaler)
	if !ok {
		return make([]byte, 0), fmt.Errorf("unrecognisable type: %q", tdef.String())
	}
	b, rest, err := um.byteUnmarshal(s)
	if err != nil {
		return b, err
	}
	if rest != "" {
		return b, fmt.Errorf("string not consumed; remaining: %q", preview(rest))
	}
	return b, err
}

func (it *intType) byteUnmarshal(s string) ([]byte, string, error) {
	s0 := s
    s = strings.TrimFunc(s, unicode.IsSpace)
    if len(s) == 0 {
        return []byte{0}, s0, fmt.Errorf("empty input")
    }

	tok, rest, ok := readNumberToken(s, false)
    if !ok {
        return []byte{0}, s0, fmt.Errorf("expected int at %q", preview(s))
    }
    if num, err := strconv.ParseInt(tok, 10, 64); err != nil {
        return []byte{0}, s0, fmt.Errorf("invalid int %q: %v", tok, err)
    } else {
		b := make([]byte, 8)
		binary.LittleEndian.PutUint64(b, uint64(num))
    	return b, rest, nil
	}
}

func (ft *floatType) byteUnmarshal(s string) ([]byte, string, error) {
	s0 := s
    s = strings.TrimFunc(s, unicode.IsSpace)
    if len(s) == 0 {
        return []byte{0}, s0, fmt.Errorf("empty input")
    }

	tok, rest, ok := readNumberToken(s, true)
    if !ok {
        return []byte{0}, s0, fmt.Errorf("expected float at %q", preview(s))
    }
    if num, err := strconv.ParseFloat(tok, 64); err != nil {
        return []byte{0}, s0, fmt.Errorf("invalid float %q: %v", tok, err)
    } else {
    	b := make([]byte, 8)
		binary.LittleEndian.PutUint64(b, uint64(num))
		return b, rest, nil
    }
}

func (bt *boolType) byteUnmarshal(s string) ([]byte, string, error) {
	s0 := s
    s = strings.TrimFunc(s, unicode.IsSpace)
    if len(s) == 0 {
        return []byte{0}, s0, fmt.Errorf("empty input")
    }

	if strings.HasPrefix(s, "true") && boundary(s, 4) {
        return []byte{1}, strings.TrimLeftFunc(s[4:], unicode.IsSpace), nil
    }
    if strings.HasPrefix(s, "false") && boundary(s, 5) {
        return []byte{0}, strings.TrimLeftFunc(s[5:], unicode.IsSpace), nil
    }
    return []byte{0}, s0, fmt.Errorf("expected bool at %q", preview(s))
}

func (st *stringType) byteUnmarshal(s string) ([]byte, string, error) {
	s0 := s
    s = strings.TrimFunc(s, unicode.IsSpace)
    if len(s) == 0 {
        return []byte(""), s0, fmt.Errorf("empty input")
    }

	tok, rest, ok := readStringToken(s)
    if !ok {
        return []byte(""), s0, fmt.Errorf("expected string at %q", preview(s))
    }
    return []byte(tok), rest, nil
} 

func (lt *listType) byteUnmarshal(s string) ([]byte, string, error) {
	s0 := s
    s = strings.TrimFunc(s, unicode.IsSpace)
    if len(s) == 0 {
        return make([]byte, 0), s0, fmt.Errorf("empty input")
    }

	if s[0] != '[' {
		return make([]byte, 0), s0, fmt.Errorf("expected list at %q", preview(s))
	}
	s = strings.TrimFunc(s[1:], unicode.IsSpace)

	if len(s) > 0 && s[0] == ']' {
		return /*newListExpr(nil, lt.line)*/make([]byte, 0), strings.TrimFunc(s[1:], unicode.IsSpace), nil
	}

	var bs []byte
	for {
		if len(s) == 0 {
			return make([]byte, 0), s, fmt.Errorf("unterminated list")
		}

		um, ok := lt.tdef.(byteUnmarshaler)
		if !ok {
			return make([]byte, 0), s0, fmt.Errorf("unrecognisable at %q", preview(s))
		} 
		b, rest, err := um.byteUnmarshal(s)
		if err != nil {
			return make([]byte, 0), rest, err
		}
		bs = append(bs, b...)

		s = strings.TrimFunc(rest, unicode.IsSpace)
		if len(s) == 0 {
			return make([]byte, 0), s, fmt.Errorf("unterminated list")
		}

		switch s[0] {
			case ',':
				s = strings.TrimFunc(s[1:], unicode.IsSpace)
			case ']':
				return bs, strings.TrimFunc(s[1:], unicode.IsSpace), nil
			default:
				return make([]byte, 0), s, fmt.Errorf("expected ',' or ']' at %q", preview(s))
		}
	}
}

func (rt *recordType) byteUnmarshal(s string) ([]byte, string, error) {
	expr, rest, err := rt.unmarshal(s)
	if err != nil {
		return make([]byte, 0), rest, err
	}
	v := expr.String()
	return []byte(v), rest, err
}