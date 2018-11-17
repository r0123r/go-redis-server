package redis

// Translate the 'KEYS' argument ('foo*', 'f??', &c.) into a regexp.

import (
	"bytes"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"
)

// patternRE compiles a KEYS argument to a regexp. Returns nil if the given
// pattern will never match anything.
// The general strategy is to sandwich all non-meta characters between \Q...\E.
func patternRE(k string) *regexp.Regexp {
	re := bytes.Buffer{}
	re.WriteString(`^\Q`)
	for i := 0; i < len(k); i++ {
		p := k[i]
		switch p {
		case '*':
			re.WriteString(`\E.*\Q`)
		case '?':
			re.WriteString(`\E.\Q`)
		case '[':
			charClass := bytes.Buffer{}
			i++
			for ; i < len(k); i++ {
				if k[i] == ']' {
					break
				}
				if k[i] == '\\' {
					if i == len(k)-1 {
						// Ends with a '\'. U-huh.
						return nil
					}
					charClass.WriteByte(k[i])
					i++
					charClass.WriteByte(k[i])
					continue
				}
				charClass.WriteByte(k[i])
			}
			if charClass.Len() == 0 {
				// '[]' is valid in Redis, but matches nothing.
				return nil
			}
			re.WriteString(`\E[`)
			re.Write(charClass.Bytes())
			re.WriteString(`]\Q`)

		case '\\':
			if i == len(k)-1 {
				// Ends with a '\'. U-huh.
				return nil
			}
			// Forget the \, keep the next char.
			i++
			re.WriteByte(k[i])
			continue
		default:
			re.WriteByte(p)
		}
	}
	re.WriteString(`\E$`)
	return regexp.MustCompile(re.String())
}

func (h *DefaultHandler) MGet(keys ...string) ([][]byte, error) {
	if h.Database == nil || h.values == nil {
		return nil, nil
	}
	rez := make([][]byte, len(keys))
	for i, key := range keys {
		rez[i] = h.values[key]
	}
	return rez, nil
}

func (h *DefaultHandler) MSet(args ...[]byte) error {
	if h.Database == nil {
		h.Database = NewDatabase(nil)
	}

	if len(args)%2 != 0 {
		return fmt.Errorf("not values")
	}
	for len(args) > 0 {
		key, value := args[0], args[1]
		args = args[2:]

		h.values[string(key)] = value
	}

	return nil
}
func (h *DefaultHandler) Keys(pattern string) ([][]byte, error) {
	if h.Database == nil {
		h.Database = NewDatabase(nil)
	}
	res := make([][]byte, 0)
	re := patternRE(pattern)
	if re == nil {
		return nil, fmt.Errorf("pattern - invalid format")
	} else {
		for key, _ := range h.values {
			if !re.MatchString(key) {
				continue
			}
			res = append(res, []byte(key))
		}
		for key, _ := range h.hvalues {
			if !re.MatchString(key) {
				continue
			}
			res = append(res, []byte(key))
		}
		for key, _ := range h.brstack {
			if !re.MatchString(key) {
				continue
			}
			res = append(res, []byte(key))
		}
		for key, _ := range h.orderedSet {
			if !re.MatchString(key) {
				continue
			}
			res = append(res, []byte(key))
		}

	}

	return res, nil
}
func (h *DefaultHandler) FlushAll() error {
	if h.Database == nil {
		return nil
	}
	for _, db := range h.Database.children {
		db.values = make(HashValue)
		db.hvalues = make(HashHash)
		db.brstack = make(HashBrStack)

	}
	return nil
}
func (h *DefaultHandler) FlushDB() error {
	if h.Database == nil {
		return nil
	}
	h.values = make(HashValue)
	h.hvalues = make(HashHash)
	h.brstack = make(HashBrStack)
	return nil
}

func (h *DefaultHandler) Ttl(key string) (int, error) {

	if h.Database == nil {
		return 0, nil
	}

	if ok, _ := h.Exists(key); ok != 1 {
		// No such key
		return -2, nil

	}

	v, ok := h.ttl[key]
	if !ok {
		// no expire value
		return -1, nil

	}
	return int(v.Sub(time.Now()).Seconds()), nil

}
func (h *DefaultHandler) Time() ([][]byte, error) {
	now := time.Now().UTC()
	ret := [][]byte{
		[]byte(strconv.Itoa(int(now.Unix()))),
		[]byte(strconv.Itoa(int(now.UnixNano() - now.Unix()*1000000000))),
	}
	return ret, nil
}
func (h *DefaultHandler) Info(key ...string) ([]byte, error) {
	var b = bytes.NewBuffer(make([]byte, 0))
	fmt.Fprint(b, "redis_version:gopex", "\r\n")
	fmt.Fprint(b, "used_memory:", 1, "\r\n")
	return b.Bytes(), nil
}
func (h *DefaultHandler) DbSize() (int, error) {

	if h.Database == nil {
		return 0, nil
	}
	size := 0
	size += len(h.values)
	size += len(h.hvalues)
	size += len(h.brstack)
	return size, nil
}
func (h *DefaultHandler) Config(key, arg string) ([][]byte, error) {

	if strings.ToLower(key) == "get" && strings.ToLower(arg) == "databases" {
		return [][]byte{[]byte(arg), []byte("16")}, nil
	} else {
		println("Config ", key, " ", arg)
	}
	return nil, nil

}
func (h *DefaultHandler) Scan(args ...string) ([]interface{}, error) {

	if len(args) < 1 {
		return nil, fmt.Errorf("args < 1")

	}

	//	cursor, err := strconv.Atoi(args[0])
	//	if err != nil {
	//		return nil, fmt.Errorf("Invalid Cursor")
	//	}
	if h.Database == nil {
		return nil, nil
	}

	args = args[1:]

	// MATCH and COUNT options
	var withMatch bool
	var match string
	for len(args) > 0 {
		if strings.ToLower(args[0]) == "count" {
			// we do nothing with count
			if len(args) < 2 {
				return nil, fmt.Errorf("Syntax Error")
			}
			if _, err := strconv.Atoi(args[1]); err != nil {
				return nil, fmt.Errorf("Invalid Int")
			}
			args = args[2:]
			continue
		}
		if strings.ToLower(args[0]) == "match" {
			if len(args) < 2 {
				return nil, fmt.Errorf("Syntax Error")
			}
			withMatch = true
			match, args = args[1], args[2:]
			continue
		}
		return nil, fmt.Errorf("Syntax Error")
	}

	if !withMatch {
		match = "*"
	}
	res := []interface{}{}
	re := patternRE(match)
	if re != nil {
		for key, _ := range h.values {
			if !re.MatchString(key) {
				continue
			}
			res = append(res, key)
		}
		for key, _ := range h.hvalues {
			if !re.MatchString(key) {
				continue
			}
			res = append(res, key)
		}
		for key, _ := range h.brstack {
			if !re.MatchString(key) {
				continue
			}
			res = append(res, key)
		}
		for key, _ := range h.orderedSet {
			if !re.MatchString(key) {
				continue
			}
			res = append(res, key)
		}

	}
	ret := []interface{}{"0", res}
	return ret, nil
}
func (h *DefaultHandler) Type(key string) (interface{}, error) {

	if h.Database == nil {
		return nil, nil
	}
	if _, ok := h.values[key]; ok {
		return "string", nil
	}
	if _, ok := h.hvalues[key]; ok {
		return "hash", nil

	}
	if _, ok := h.brstack[key]; ok {
		return "list", nil

	}
	if _, ok := h.orderedSet[key]; ok {
		return "zset", nil
	}
	return "none", nil
}
func (h *DefaultHandler) Hlen(key string) (interface{}, error) {
	if h.Database == nil {
		return nil, nil
	}
	if v, ok := h.hvalues[key]; ok {
		return len(v), nil
	}
	return 0, nil
}
func (h *DefaultHandler) Llen(key string) (interface{}, error) {
	if h.Database == nil {
		return nil, nil
	}
	if v, ok := h.brstack[key]; ok {
		return v.Len(), nil
	}
	return 0, nil
}
func (h *DefaultHandler) Lset(key string, ind int, value []byte) (interface{}, error) {
	if h.Database == nil {
		return nil, nil
	}
	if v, ok := h.brstack[key]; ok {
		v.SetIndex(ind, value)
		return "OK", nil
	}
	return nil, nil
}
func (h *DefaultHandler) Lrem(key string, count int, value []byte) (interface{}, error) {
	if h.Database == nil {
		return nil, nil
	}
	if v, ok := h.brstack[key]; ok {
		rez := v.FilterRem(value, count)
		return rez, nil
	}
	return nil, nil
}
func (h *DefaultHandler) Zcard(key string) (int, error) {
	if h.Database == nil {
		return 0, nil
	}
	if v, ok := h.orderedSet[key]; ok {
		return len(v.elements), nil
	}
	return 0, nil
}
func (h *DefaultHandler) Zscore(key, val string) (interface{}, error) {
	if h.Database == nil {
		return nil, nil
	}
	if v, ok := h.orderedSet[key]; ok {
		res := v.Score(val)
		switch r := res.(type) {
		case int:
			return strconv.Itoa(r), nil
		default:
			return nil, nil
		}
	}
	return nil, nil
}
