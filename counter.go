package hitcounter

import (
	"errors"
	"fmt"
	"strings"
	"sync/atomic"
	"time"
)

type HitCounter interface {
	AddHit()
	GetHits() uint64
	String() string
}

// Now can be used to inject a different means of getting the current time
var Now func() time.Time = time.Now

var ErrInvalidDuration = errors.New("counter duration must be a multiple of its resolution")

type slot struct {
	time time.Time
	hits uint64
}

type slots []*slot

func (s slots) String() string {
	ss := make([]string, len(s))
	for i, s := range s {
		ss[i] = s.String()
	}
	return fmt.Sprintf("[ %s ]", strings.Join(ss, ", "))
}

func NewSlot(t time.Time) *slot {
	return &slot{time: t}
}

// String returns a slot's string representation, which presents the number of hits
// that occurred within the time interval represented by the slot.
func (s *slot) String() string {
	return fmt.Sprintf("%d hits at %s", s.hits, s.time)
}

func (s *slot) AddHit() {
	atomic.AddUint64(&s.hits, 1)
}

func (s *slot) Time() time.Time {
	return s.time
}

func (s *slot) Hits() uint64 {
	return s.hits
}
