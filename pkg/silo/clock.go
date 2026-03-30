package silo

import "time"

// Clock abstracts time for deterministic testing.
type Clock interface {
	Now() time.Time
	NewTicker(d time.Duration) Ticker
}

// Ticker abstracts time.Ticker for testing.
type Ticker interface {
	C() <-chan time.Time
	Stop()
}

// RealClock uses the standard library time functions.
type RealClock struct{}

func (RealClock) Now() time.Time                   { return time.Now() }
func (RealClock) NewTicker(d time.Duration) Ticker  { return &realTicker{time.NewTicker(d)} }

type realTicker struct{ t *time.Ticker }

func (r *realTicker) C() <-chan time.Time { return r.t.C }
func (r *realTicker) Stop()              { r.t.Stop() }
