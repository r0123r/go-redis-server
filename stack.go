package redis

import (
	"bytes"
	"sync"
)

type Stack struct {
	sync.Mutex
	Key   string
	stack [][]byte
	Chan  chan *Stack
}

func (s *Stack) PopBack() []byte {
	s.Lock()
	defer s.Unlock()

	if s.stack == nil || len(s.stack) == 0 {
		return nil
	}

	var ret []byte
	if len(s.stack)-1 == 0 {
		ret, s.stack = s.stack[0], [][]byte{}
	} else {
		ret, s.stack = s.stack[len(s.stack)-1], s.stack[:len(s.stack)-1]
	}
	return ret
}
func (s *Stack) PushBackLite(vals ...[]byte) {
	for _, val := range vals {
		s.stack = append(s.stack, val)
	}
}
func (s *Stack) PushBack(val []byte) {
	s.Lock()
	defer s.Unlock()

	if s.stack == nil {
		s.stack = [][]byte{}
	}

	go func() {
		if s.Chan != nil {
			s.Chan <- s
		}
	}()
	s.stack = append(s.stack, val)
}

func (s *Stack) PopFront() []byte {
	s.Lock()
	defer s.Unlock()

	if s.stack == nil || len(s.stack) == 0 {
		return nil
	}

	var ret []byte
	if len(s.stack)-1 == 0 {
		ret, s.stack = s.stack[0], [][]byte{}
	} else {
		ret, s.stack = s.stack[0], s.stack[1:]
	}
	return ret
}

func (s *Stack) PushFront(val []byte) {
	s.Lock()
	defer s.Unlock()

	if s.stack == nil {
		s.stack = [][]byte{}
	}

	s.stack = append([][]byte{val}, s.stack...)
	go func() {
		if s.Chan != nil {
			s.Chan <- s
		}
	}()
}

// GetIndex return the element at the requested index.
// If no element correspond, return nil.
func (s *Stack) GetIndex(index int) []byte {
	s.Lock()
	defer s.Unlock()

	if index < 0 {
		if len(s.stack)+index >= 0 {
			return s.stack[len(s.stack)+index]
		}
		return nil
	}
	if len(s.stack) > index {
		return s.stack[index]
	}
	return nil
}

func (s *Stack) Len() int {
	s.Lock()
	defer s.Unlock()
	return len(s.stack)
}

func NewStack(key string) *Stack {
	return &Stack{
		stack: [][]byte{},
		Chan:  make(chan *Stack),
		Key:   key,
	}
}
func (s *Stack) SetIndex(index int, val []byte) {
	s.Lock()
	defer s.Unlock()

	if index < 0 {
		if len(s.stack)+index >= 0 {
			s.stack[len(s.stack)+index] = val
		}
	} else if len(s.stack) > index {
		s.stack[index] = val
	}

}
func (s *Stack) DelIndex(index int) {
	s.Lock()
	defer s.Unlock()

	if index < 0 {
		i := len(s.stack) + index
		if i >= 0 {
			s.stack = append(s.stack[:i], s.stack[i+1:]...)
		}
	} else if len(s.stack) > index {
		s.stack = append(s.stack[:index], s.stack[index+1:]...)
	}

}

//Filtering without allocating
func (s *Stack) FilterRem(val []byte, count int) int {
	s.Lock()
	defer s.Unlock()
	b := s.stack[:0]
	c := 0
	for _, x := range s.stack {
		if bytes.Equal(x, val) {
			c++
		} else {
			b = append(b, x)
		}
	}
	if c > 0 {
		s.stack = s.stack[:len(s.stack)-c]
	}
	return c
}
