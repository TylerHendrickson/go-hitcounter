package hitcounter

import (
	"sync"
	"sync/atomic"
	"time"
)

// ImmediateHitCounter is an implementation of a hit counter that uses a fixed amount of memory
// that is allocated when the ImmediateHitCounter is created by calling NewImmediateHitCounter().
type ImmediateHitCounter struct {
	slots slots
	res   time.Duration
	mux   sync.Mutex
}

func (c *ImmediateHitCounter) now() time.Time {
	return Now().Truncate(c.res)
}

func (c *ImmediateHitCounter) String() string {
	return c.slots.String()
}

// GetDuration returns the configured duration of the Counter.
func (c *ImmediateHitCounter) GetDuration() time.Duration {
	return time.Duration(len(c.slots)) * c.res
}

// NewImmediateHitCounter returns a pointer to a new Counter with a rolling expiration window duration d,
// with resolution of r. For optimized memory, r should be the largest value that d is divisible by.
// When the given duration is not divisible by the given resolution, no Counter is created
// and an error is returned.
//
// Example: NewImmediateHitCounter(5*time.Minute, time.Minute) creates a counter that tracks hits over
// a rolling 5-minute period.
func NewImmediateHitCounter(d time.Duration, r time.Duration) (*ImmediateHitCounter, error) {
	if d <= r || d%r != 0 {
		return nil, ErrInvalidDuration
	}

	numSlots := d / r
	c := &ImmediateHitCounter{slots: make([]*slot, numSlots), res: r}
	fillTime := c.now()
	for i := 0; i < int(numSlots); i++ {
		c.slots[i] = &slot{time: fillTime}
		fillTime = fillTime.Add(-r)
	}
	return c, nil
}

// GetHits() returns the total number of hits that occurred within the Counter's configured duration.
func (c *ImmediateHitCounter) GetHits() (total uint64) {
	notValidBefore := c.now().Add(-c.res * time.Duration(len(c.slots)))
	for _, slot := range c.slots {
		if !slot.time.Before(notValidBefore) {
			total += slot.hits
		}
	}
	return
}

// AddHit atomically increments the number of hits for the currently-tracked interval,
// which is determined by the Counter's configured resolution.
// The recorded hit will be included in calls to GetHits() until the Counter's configured
// duration has elapsed.
func (c *ImmediateHitCounter) AddHit() {
	now := c.now()
	c.maybeShiftIn(c.now())
	for _, s := range c.slots {
		if s.time.Equal(now) {
			atomic.AddUint64(&s.hits, 1)
			break
		}
	}
}

func (c *ImmediateHitCounter) maybeShiftIn(t time.Time) {
	c.mux.Lock()
	defer c.mux.Unlock()
	if c.slots[0].time.Before(t) {
		for i := len(c.slots) - 1; i > 0; i-- {
			c.slots[i] = c.slots[i-1]
		}
		c.slots[0] = &slot{time: t, hits: 0}
	}
}
