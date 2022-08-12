package counter_test

import (
	"errors"
	"fmt"
	"math/rand"
	"testing"
	"time"

	counter "github.com/TylerHendrickson/go-hitcounter"
)

func TestNewExpiringCounter(t *testing.T) {
	for _, tt := range []struct {
		name                 string
		duration, resolution time.Duration
		expectedError        error
	}{
		{"10 second timer", time.Second * 10, time.Second, nil},
		{"5 minute timer", time.Minute * 5, time.Second, nil},
		{
			"invalid timer config with duration shorter than resolution",
			time.Second, time.Minute, counter.ErrInvalidDuration,
		},
		{
			"invalid timer config with duration indivisuable by resolution",
			time.Second * 10, time.Second * 7, counter.ErrInvalidDuration,
		},
		{
			"valid yet efficiently bespoke timer config",
			time.Second * 10, time.Second * 2, nil,
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			c, err := counter.NewExpiringCounter(tt.duration, tt.resolution)
			if tt.expectedError != nil {
				if !errors.Is(err, tt.expectedError) {
					t.Errorf("expected %q error but got %v", tt.expectedError, err)
				}
			} else if err != nil {
				t.Errorf("unexpected error creating new counter: %q", err)
			} else {
				if configuredDuration := c.GetDuration(); configuredDuration != tt.duration {
					t.Errorf("expected counter with duration %s but got %s",
						tt.duration, configuredDuration)
				}
			}
		})
	}
}

func TestRollingTicksWithVariableHits(t *testing.T) {
	for ti, tt := range []struct {
		duration    int
		hitsPerTick []uint64
	}{
		{5, []uint64{1, 2, 3, 4, 5}},                    // last 5 seconds: 1+2+3+4+5=15
		{5, []uint64{1, 2, 3, 4, 5, 6, 7, 8, 9, 10}},    // last 5 seconds: 6+7+8+9+10=40
		{3, []uint64{3, 5, 2, 8, 92, 4, 6, 7, 1, 9, 3}}, // last 3 seconds: 1+9+3=13
	} {
		expected := uint64Sum(tt.hitsPerTick[len(tt.hitsPerTick)-tt.duration : len(tt.hitsPerTick)])
		testName := fmt.Sprintf("%d: expect %d hits in last %d seconds", ti, expected, tt.duration)

		t.Run(testName, func(t *testing.T) {
			restoreClockNow := counter.Now
			t.Cleanup(func() { counter.Now = restoreClockNow })
			startTime := counter.Now()
			mockTime := startTime
			counter.Now = func() time.Time {
				return mockTime
			}

			c, err := counter.NewExpiringCounter(time.Second*time.Duration(tt.duration), time.Second)
			if err != nil {
				t.Errorf("Unexpected error: %s", err)
				t.FailNow()
			}

			for offset, hits := range tt.hitsPerTick {
				mockTime = startTime.Add(time.Duration(offset) * time.Second)
				for i := uint64(0); i < hits; i++ {
					c.AddHit()
				}
			}

			if result := c.GetHits(); result != expected {
				t.Errorf("expected %v but got %v in counter %s", expected, result, c)
			}
		})
	}
}

func TestOutOfOrderHits(t *testing.T) {
	for ti, tt := range []struct {
		duration    int
		hitsPerTick []uint64
	}{
		{5, []uint64{1, 2, 3, 4, 5}},                    // last 5 seconds: 1+2+3+4+5=15
		{5, []uint64{1, 2, 3, 4, 5, 6, 7, 8, 9, 10}},    // last 5 seconds: 6+7+8+9+10=40
		{3, []uint64{3, 5, 2, 8, 92, 4, 6, 7, 1, 9, 3}}, // last 3 seconds: 1+9+3=13
	} {
		expected := uint64Sum(tt.hitsPerTick[len(tt.hitsPerTick)-tt.duration : len(tt.hitsPerTick)])
		testName := fmt.Sprintf("%d: expect %d hits in last %d seconds", ti, expected, tt.duration)
		t.Run(testName, func(t *testing.T) {
			restoreClockNow := counter.Now
			t.Cleanup(func() { counter.Now = restoreClockNow })
			mockTime := counter.Now().Truncate(time.Second)
			simTime := mockTime.Add(-time.Duration(len(tt.hitsPerTick)) * time.Second)
			counter.Now = func() time.Time { return mockTime }

			hitMoments := make([]time.Time, 0)
			for _, numHits := range tt.hitsPerTick {
				simTime = simTime.Add(time.Second)
				for i := 0; i < int(numHits); i++ {
					hitMoments = append(hitMoments, simTime)
				}
			}
			shuffleTimes(hitMoments)

			c, err := counter.NewExpiringCounter(time.Second*time.Duration(tt.duration), time.Second)
			if err != nil {
				t.Errorf("Unexpected error: %s", err)
				t.FailNow()
			}

			for _, t := range hitMoments {
				c.AddHitAtTime(t)
			}
			if result := c.GetHits(); result != expected {
				t.Errorf("expected %v but got %v in counter %s", expected, result, c)
			}
		})
	}
}

func uint64Sum(elems []uint64) (total uint64) {
	for _, i := range elems {
		total += i
	}
	return
}

func shuffleTimes(times []time.Time) {
	rand.Seed(time.Now().UnixNano())
	rand.Shuffle(len(times), func(i, j int) {
		times[i], times[j] = times[j], times[i]
	})
}
