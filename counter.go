package counter

import (
	"errors"
	"fmt"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

// Now can be replaced to inject a different means of getting the current time
var Now func() time.Time = time.Now

// A slot tracks hits that occurred at a certain (resolved/rounded) timestamp.
// It represents the smallest resolution by which hits may be tracked by a counter.
type slot struct {
	time time.Time
	hits uint64
}

// NewSlot returns a pointer to a new slot for tracking hits bounded to a resolved time t.
// The new counter will be initialized with zero hits.
func NewSlot(t time.Time) *slot {
	return &slot{time: t}
}

// String returns a slot's string representation, which presents the number of hits
// that occurred within the time interval represented by the slot.
func (s *slot) String() string {
	return fmt.Sprintf("%d hits at %s", s.hits, s.time)
}

// AddHit atomically increases the number of hits for its slot
func (s *slot) AddHit() {
	atomic.AddUint64(&s.hits, 1)
}

func (s *slot) GetTime() time.Time {
	return s.time
}

func (s *slot) GetHits() uint64 {
	return s.hits
}

type slots []*slot

func (s slots) String() string {
	ss := make([]string, len(s))
	for i, s := range s {
		ss[i] = s.String()
	}
	return fmt.Sprintf("[ %s ]", strings.Join(ss, ", "))
}

var ErrInvalidDuration = errors.New("counter duration must be a multiple of its resolution")

type ExpiringCounter struct {
	slots slots
	res   time.Duration
	mux   sync.Mutex
}

func (c *ExpiringCounter) now() time.Time {
	return Now().Truncate(c.res)
}

func (c *ExpiringCounter) String() string {
	return c.slots.String()
}

// GetDuration returns the configured duration of the Counter.
func (c *ExpiringCounter) GetDuration() time.Duration {
	return time.Duration(len(c.slots)) * c.res
}

// NewExpiringCounter returns a pointer to a new ExpiringCounter with a rolling expiration window duration d,
// with resolution of r. For optimized memory, r should be the largest value that d is divisible by.
// When the given duration is not divisible by the given resolution, no ExpiringCounter is created
// and an error is returned.
//
// Example: NewExpiringCounter(5*time.Minute, time.Minute) creates a counter that tracks hits over
// a rolling 5-minute period.
func NewExpiringCounter(d time.Duration, r time.Duration) (*ExpiringCounter, error) {
	if d <= r || d%r != 0 {
		return nil, ErrInvalidDuration
	}

	numSlots := d / r
	c := &ExpiringCounter{slots: make([]*slot, numSlots), res: r}
	fillTime := c.now()
	for i := 0; i < int(numSlots); i++ {
		c.slots[i] = NewSlot(fillTime)
		fillTime = fillTime.Add(-r)
	}
	return c, nil
}

// GetHits() returns the total number of hits that occurred within the Counter's configured duration.
func (c *ExpiringCounter) GetHits() (total uint64) {
	notValidBefore := c.now().Add(-c.res * time.Duration(len(c.slots)))
	for _, slot := range c.slots {
		if !slot.GetTime().Before(notValidBefore) {
			total += slot.GetHits()
		}
	}
	return
}

// AddHit atomically increments the number of hits for the current time,
// which is determined by the Counter's configured resolution.
// The recorded hit will be included in calls to GetHits() until the Counter's configured
// duration has elapsed.
func (c *ExpiringCounter) AddHit() {
	c.AddHitAtTime(c.now())
}

// AddHitAtTime is like AddHit, but takes a discrete time instead of inferring the time
// of the hit based on the current time. This is more useful for adressing latency concerns
// between the "real" moment of hit occurrence and when the hit is recorded (e.g. if a webserver
// is using the Counter to track page hits and calls to the Counter are deferred).
// Note that in very high-latency scenarios, the hit will not be recorded if the given Time
// is beyond the configured duration for the Counter.
func (c *ExpiringCounter) AddHitAtTime(t time.Time) {
	t = t.Truncate(c.res)
	for i := c.maybeInsertSlot(t); i >= 0 && i < len(c.slots); i++ {
		if s := c.slots[i]; s.time.Equal(t) {
			s.AddHit()
			break
		}
	}
}

func (c *ExpiringCounter) maybeInsertSlot(t time.Time) int {
	c.mux.Lock()
	defer c.mux.Unlock()

	if c.slots[0].time.Equal(t) {
		// Insertion time is already the latest slot time
		return 0
	}

	if c.slots[0].time.Before(t) {
		// Insertion time is more recent than the latest slot time
		// Insert a new slot at the front of the slots
		for i := len(c.slots) - 1; i > 0; i-- {
			c.slots[i] = c.slots[i-1]
		}
		c.slots[0] = NewSlot(t)
		return 0
	}

	if t.Before(c.slots[len(c.slots)-1].time) {
		// Given time is too old for the counter
		return -1
	}

	// Figure out where to insert a new slot
	insertPos := 1
	for insertPos < len(c.slots)-1 {
		if t.After(c.slots[insertPos].time) {
			break
		}
		insertPos++
	}

	// Shift-right all slots after the determined insert position
	for i := len(c.slots) - 1; i > insertPos; i-- {
		c.slots[i] = c.slots[i-1]
	}

	// Insert the new slot
	c.slots[insertPos] = NewSlot(t)
	return insertPos
}
