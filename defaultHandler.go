package redis

import (
	"fmt"
	"reflect"
	"strconv"
	"time"
)

type (
	HashValue      map[string][]byte
	HashHash       map[string]HashValue
	HashSub        map[string][]*ChannelWriter
	HashBrStack    map[string]*Stack
	HashTtl        map[string]time.Time
	HashOrderedSet map[string]*OrderedSet
)

type Database struct {
	values  HashValue
	hvalues HashHash
	brstack HashBrStack
	ttl     HashTtl

	orderedSet HashOrderedSet
}

func NewDatabase(parent *Database) *Database {
	db := &Database{
		values:  make(HashValue),
		hvalues: make(HashHash),

		brstack:    make(HashBrStack),
		ttl:        make(HashTtl),
		orderedSet: make(HashOrderedSet),
	}
	//db.children[0] = db
	go func(db *Database) {
		for {
			now := time.Now()
			for key, val := range db.ttl {
				if now.Sub(val).Seconds() >= 0 {
					delete(db.values, key)
					delete(db.hvalues, key)
					delete(db.brstack, key)
					delete(db.orderedSet, key)
					delete(db.ttl, key)
				}
			}
			time.Sleep(time.Second)
		}
	}(db)

	return db
}

type DefaultHandler struct {
	*Database
	CurrentDb int
	dbs       map[int]*Database
	sub       HashSub
}

func (h *DefaultHandler) Rpush(key string, value []byte, values ...[]byte) (int, error) {
	values = append([][]byte{value}, values...)
	h.Database = h.dbs[h.CurrentDb]

	if _, exists := h.brstack[key]; !exists {
		h.brstack[key] = NewStack(key)
	}
	for _, value := range values {
		h.brstack[key].PushBack(value)
	}
	return h.brstack[key].Len(), nil
}

func (h *DefaultHandler) Brpop(key string, keys ...string) (data [][]byte, err error) {
	keys = append([]string{key}, keys...)
	h.Database = h.dbs[h.CurrentDb]

	if len(keys) == 0 {
		return nil, ErrParseTimeout
	}

	timeout, err := strconv.Atoi(keys[len(keys)-1])
	if err != nil {
		return nil, ErrParseTimeout
	}
	keys = keys[:len(keys)-1]

	var timeoutChan <-chan time.Time
	if timeout > 0 {
		timeoutChan = time.After(time.Duration(timeout) * time.Second)
	} else {
		timeoutChan = make(chan time.Time)
	}

	finishedChan := make(chan struct{})
	go func() {
		defer close(finishedChan)
		selectCases := []reflect.SelectCase{}
		for _, k := range keys {
			key := string(k)
			if _, exists := h.brstack[key]; !exists {
				h.brstack[key] = NewStack(k)
			}
			selectCases = append(selectCases, reflect.SelectCase{
				Dir:  reflect.SelectRecv,
				Chan: reflect.ValueOf(h.brstack[key].Chan),
			})
		}
		_, recv, _ := reflect.Select(selectCases)
		s, ok := recv.Interface().(*Stack)
		if !ok {
			err = fmt.Errorf("Impossible to retrieve data. Wrong type.")
			return
		}
		data = [][]byte{[]byte(s.Key), s.PopBack()}
	}()

	select {
	case <-finishedChan:
		return data, err
	case <-timeoutChan:
		return nil, nil
	}
	return nil, nil
}

func (h *DefaultHandler) Lrange(key string, start, stop int) ([][]byte, error) {
	h.Database = h.dbs[h.CurrentDb]
	if _, exists := h.brstack[key]; !exists {
		h.brstack[key] = NewStack(key)
	}

	if start < 0 {
		if start = h.brstack[key].Len() + start; start < 0 {
			start = 0
		}
	}

	if stop < 0 {
		if stop = h.brstack[key].Len() + stop; stop < 0 {
			stop = 0
		}
	}

	var ret [][]byte
	for i := start; i <= stop; i++ {
		if val := h.brstack[key].GetIndex(i); val != nil {
			ret = append(ret, val)
		}
	}
	return ret, nil
}

func (h *DefaultHandler) Lindex(key string, index int) ([]byte, error) {
	h.Database = h.dbs[h.CurrentDb]
	if _, exists := h.brstack[key]; !exists {
		h.brstack[key] = NewStack(key)
	}
	return h.brstack[key].GetIndex(index), nil
}

func (h *DefaultHandler) Lpush(key string, value []byte, values ...[]byte) (int, error) {
	values = append([][]byte{value}, values...)
	h.Database = h.dbs[h.CurrentDb]
	if _, exists := h.brstack[key]; !exists {
		h.brstack[key] = NewStack(key)
	}
	for _, value := range values {
		h.brstack[key].PushFront(value)
	}
	return h.brstack[key].Len(), nil
}

func (h *DefaultHandler) Blpop(key string, keys ...string) (data [][]byte, err error) {
	keys = append([]string{key}, keys...)
	h.Database = h.dbs[h.CurrentDb]

	if len(keys) == 0 {
		return nil, ErrParseTimeout
	}

	timeout, err := strconv.Atoi(keys[len(keys)-1])
	if err != nil {
		return nil, ErrParseTimeout
	}
	keys = keys[:len(keys)-1]

	var timeoutChan <-chan time.Time
	if timeout > 0 {
		timeoutChan = time.After(time.Duration(timeout) * time.Second)
	} else {
		timeoutChan = make(chan time.Time)
	}

	finishedChan := make(chan struct{})

	go func() {
		defer close(finishedChan)
		selectCases := []reflect.SelectCase{}
		for _, k := range keys {
			key := string(k)
			if _, exists := h.brstack[key]; !exists {
				h.brstack[key] = NewStack(k)
			}
			selectCases = append(selectCases, reflect.SelectCase{
				Dir:  reflect.SelectRecv,
				Chan: reflect.ValueOf(h.brstack[key].Chan),
			})
		}
		_, recv, _ := reflect.Select(selectCases)
		s, ok := recv.Interface().(*Stack)
		if !ok {
			err = fmt.Errorf("Impossible to retrieve data. Wrong type.")
			return
		}
		data = [][]byte{[]byte(s.Key), s.PopFront()}
	}()

	select {
	case <-finishedChan:
		return data, err
	case <-timeoutChan:
		return nil, nil
	}
	return nil, nil
}

func (h *DefaultHandler) Hget(key, subkey string) ([]byte, error) {
	h.Database = h.dbs[h.CurrentDb]
	if h.hvalues == nil {
		return nil, nil
	}

	if v, exists := h.hvalues[key]; exists {
		if v, exists := v[subkey]; exists {
			return v, nil
		}
	}
	return nil, nil
}

func (h *DefaultHandler) Hset(key, subkey string, value []byte) (int, error) {
	ret := 0

	h.Database = h.dbs[h.CurrentDb]
	if h.hvalues == nil {
		h.hvalues = make(HashHash)
	}
	if _, exists := h.hvalues[key]; !exists {
		h.hvalues[key] = make(HashValue)
		ret = 1
	}

	if _, exists := h.hvalues[key][subkey]; !exists {
		ret = 1
	}

	h.hvalues[key][subkey] = value

	return ret, nil
}

func (h *DefaultHandler) Hgetall(key string) (HashValue, error) {
	h.Database = h.dbs[h.CurrentDb]
	if h.hvalues == nil {
		return nil, nil
	}
	return h.hvalues[key], nil
}

func (h *DefaultHandler) Get(key string) ([]byte, error) {
	h.Database = h.dbs[h.CurrentDb]
	if h.values == nil {
		return nil, nil
	}
	return h.values[key], nil
}

func (h *DefaultHandler) Set(key string, args ...[]byte) error {
	h.Database = h.dbs[h.CurrentDb]
	h.values[key] = args[0]

	if len(args) > 1 {

		switch string(args[1]) {
		case "ex":
			expire, err := strconv.Atoi(string(args[2]))
			if err != nil {
				break
			}
			ttl := time.Duration(expire) * time.Second
			if ttl >= 0 {
				h.ttl[key] = time.Now().Add(ttl)
			}
		}
	}

	return nil
}

func (h *DefaultHandler) Del(keys ...string) (int, error) {
	h.Database = h.dbs[h.CurrentDb]
	count := 0
	for _, k := range keys {
		if _, exists := h.values[k]; exists {
			delete(h.values, k)
			count++
		}
		if _, exists := h.hvalues[k]; exists {
			delete(h.hvalues, k)
			count++
		}

		if _, exists := h.brstack[k]; exists {
			delete(h.brstack, k)
			count++
		}
		if _, exists := h.orderedSet[k]; exists {
			delete(h.orderedSet, k)
			count++
		}
	}
	return count, nil
}

func (h *DefaultHandler) Ping() (*StatusReply, error) {
	return &StatusReply{Code: "PONG"}, nil
}

func (h *DefaultHandler) Subscribe(channels ...[]byte) (*MultiChannelWriter, error) {
	ret := &MultiChannelWriter{Chans: make([]*ChannelWriter, 0, len(channels))}
	for _, key := range channels {
		Debugf("SUBSCRIBE on %s\n", key)
		cw := &ChannelWriter{
			FirstReply: []interface{}{
				"subscribe",
				key,
				1,
			},
			Channel: make(chan []interface{}),
		}
		if h.sub[string(key)] == nil {
			h.sub[string(key)] = []*ChannelWriter{cw}
		} else {
			h.sub[string(key)] = append(h.sub[string(key)], cw)
		}
		ret.Chans = append(ret.Chans, cw)
	}
	return ret, nil
}

func (h *DefaultHandler) Publish(key string, value []byte) (int, error) {
	//	Debugf("Publishing %s on %s\n", value, key)
	v, exists := h.sub[key]
	if !exists {
		return 0, nil
	}
	i := 0
	for _, c := range v {
		select {
		case <-c.clientChan:
			delete(h.sub, key)
		case c.Channel <- []interface{}{
			"message",
			key,
			value,
		}:
			i++
		default:
		}
	}
	return i, nil
}

func (h *DefaultHandler) Select(key string) error {
	index, err := strconv.Atoi(key)
	if err != nil {
		return err
	}
	if _, exists := h.dbs[index]; !exists {
		fmt.Println("DB not exits, create ", index)
		h.dbs[index] = NewDatabase(nil)
	}
	h.Database = h.dbs[index]
	h.CurrentDb = index
	return nil
}

func (h *DefaultHandler) Monitor() (*MonitorReply, error) {
	return &MonitorReply{}, nil
}

var lock = make(chan bool, 1)

func (h *DefaultHandler) Incr(key string) (int, error) {
	h.Database = h.dbs[h.CurrentDb]
	lock <- true

	temp, _ := strconv.Atoi(string(h.values[key]))
	temp = temp + 1
	h.values[key] = []byte(strconv.Itoa(temp))

	<-lock

	return temp, nil
}

func (h *DefaultHandler) Decr(key string) (int, error) {
	h.Database = h.dbs[h.CurrentDb]
	lock <- true

	temp, _ := strconv.Atoi(string(h.values[key]))
	temp = temp - 1
	h.values[key] = []byte(strconv.Itoa(temp))

	<-lock

	return temp, nil
}

func (h *DefaultHandler) Expire(key, value string) (int, error) {
	h.Database = h.dbs[h.CurrentDb]
	i, err := strconv.Atoi(value)
	if err != nil {
		return 0, err
	}
	h.ttl[key] = time.Now().Add(time.Second * time.Duration(i))

	return 1, nil
}

func (h *DefaultHandler) Exists(keys ...string) (int, error) {
	h.Database = h.dbs[h.CurrentDb]
	c := int(0)
	for _, key := range keys {
		if _, exists := h.values[key]; exists {
			c++
		}
		if _, exists := h.hvalues[key]; exists {
			c++
		}
		if _, exists := h.brstack[key]; exists {
			c++
		}
		if _, exists := h.orderedSet[key]; exists {
			c++
		}
	}
	return c, nil
}

func (h *DefaultHandler) Zadd(key string, score int, value []byte, values ...[]byte) (int, error) {
	values = append([][]byte{value}, values...)

	h.Database = h.dbs[h.CurrentDb]

	if _, exists := h.orderedSet[key]; !exists {
		h.orderedSet[key] = NewOrderedSet()
	}

	ctr := 0
	for _, v := range values {
		ctr = ctr + h.orderedSet[key].Add(score, v)
	}

	return ctr, nil
}

func (h *DefaultHandler) Zrange(key string, min int, max int) ([][]byte, error) {
	h.Database = h.dbs[h.CurrentDb]

	if _, exists := h.orderedSet[key]; !exists {
		return [][]byte{}, nil
	}

	r := h.orderedSet[key].Range(min, max)

	return r, nil
}

func (h *DefaultHandler) Zrangebyscore(key string, min int, max int) ([][]byte, error) {
	h.Database = h.dbs[h.CurrentDb]

	if _, exists := h.orderedSet[key]; !exists {
		return [][]byte{}, nil
	}

	r := h.orderedSet[key].RangeByScore(min, max)

	return r, nil
}

func (h *DefaultHandler) Zrem(key string, value []byte, values ...[]byte) (int, error) {
	values = append([][]byte{value}, values...)

	h.Database = h.dbs[h.CurrentDb]

	if _, exists := h.orderedSet[key]; !exists {
		return 0, nil
	}

	ctr := 0
	for _, v := range values {
		ctr += h.orderedSet[key].Rem(v)
	}

	return ctr, nil
}

func (h *DefaultHandler) Zremrangebyscore(key string, min int, max int) (int, error) {
	h.Database = h.dbs[h.CurrentDb]

	if _, exists := h.orderedSet[key]; !exists {
		return 0, nil
	}

	return h.orderedSet[key].RemRangeByScore(min, max), nil
}

func NewDefaultHandler() *DefaultHandler {
	db := NewDatabase(nil)
	ret := &DefaultHandler{
		Database:  db,
		CurrentDb: 0,
		dbs:       map[int]*Database{0: db},
		sub:       make(HashSub),
	}
	return ret
}
