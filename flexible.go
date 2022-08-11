package hitcounter

import (
	"fmt"
	"strings"
	"sync"
	"time"
)

// FlexibleHitCounter is an implementation of HitCounter that uses a fixed amount of memory
// that is allocated when the FlexibleHitCounter is created by calling NewCounter(). It is
// similar to FixedHitCounter, but optimized for usage scenarios where hits are recorded
// (by calling AddHitAtTime) with explicit timestamps, possibly out of order.
type FlexibleHitCounter struct {
	slots []*slot
	res   time.Duration
	mux   sync.Mutex
}

func (c *FlexibleHitCounter) now() time.Time {
	return Now().Truncate(c.res)
}

func (c *FlexibleHitCounter) String() string {
	ss := make([]string, len(c.slots))
	for i, s := range c.slots {
		ss[i] = s.String()
	}
	return fmt.Sprintf("[ %s ]", strings.Join(ss, ", "))
}

// GetDuration returns the configured duration of the Counter.
func (c *FlexibleHitCounter) GetDuration() time.Duration {
	return time.Duration(len(c.slots)) * c.res
}

// NewFlexibleHitCounter returns a pointer to a new Counter with a rolling expiration window duration d,
// with resolution of r. For optimized memory, r should be the largest value that d is divisible by.
// When the given duration is not divisible by the given resolution, no Counter is created
// and an error is returned.
//
// Example: NewFlexibleHitCounter(5*time.Minute, time.Minute) creates a counter that tracks hits over
// a rolling 5-minute period.
func NewFlexibleHitCounter(d time.Duration, r time.Duration) (*FlexibleHitCounter, error) {
	if d <= r || d%r != 0 {
		return nil, ErrInvalidDuration
	}

	numSlots := d / r
	c := &FlexibleHitCounter{slots: make([]*slot, numSlots), res: r}
	fillTime := c.now()
	for i := 0; i < int(numSlots); i++ {
		c.slots[i] = NewSlot(fillTime)
		fillTime = fillTime.Add(-r)
	}
	return c, nil
}

// GetHits() returns the total number of hits that occurred within the Counter's configured duration.
func (c *FlexibleHitCounter) GetHits() (total uint64) {
	notValidBefore := c.now().Add(-c.res * time.Duration(len(c.slots)))
	for _, slot := range c.slots {
		if slot.Time().Before(notValidBefore) {
			break
		}
		total += slot.Hits()
	}
	return
}

// AddHit atomically increments the number of hits for the currently-tracked interval,
// which is determined by the Counter's configured resolution.
// The recorded hit will be included in calls to GetHits() until the Counter's configured
// duration has elapsed.
func (c *FlexibleHitCounter) AddHit() {
	c.AddHitAtTime(c.now())
}

// AddHitAtTime is like AddHit, but takes a discrete time instead of inferring the time
// of the hit based on the current time. This is more useful for adressing latency concerns
// between the "real" moment of hit occurrence and when the hit is recorded (e.g. if a webserver
// is using the Counter to track page hits and calls to the Counter are deferred).
// Note that in very high-latency scenarios, the hit will not be recorded if the given Time
// is beyond the configured duration for the Counter.
func (c *FlexibleHitCounter) AddHitAtTime(t time.Time) {
	for i := c.maybeShiftIn(t); i >= 0 && i < len(c.slots); i++ {
		if s := c.slots[i]; s.time.Equal(t) {
			s.AddHit()
			break
		}
	}
}

func (c *FlexibleHitCounter) maybeShiftIn(t time.Time) int {
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
