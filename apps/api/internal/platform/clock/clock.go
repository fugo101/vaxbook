// Package clock lets code read the time through an interface, so tests can move
// time forward without sleeping.
package clock

import (
	"sync"
	"time"
)

// Clock tells the current time.
type Clock interface {
	Now() time.Time
}

// Real is the system clock.
type Real struct{}

// Now returns the current time.
func (Real) Now() time.Time { return time.Now() }

// Fake is a manually advanced clock for tests. The zero value is not usable;
// create one with NewFake.
type Fake struct {
	mu  sync.Mutex
	now time.Time
}

// NewFake returns a Fake clock set to t.
func NewFake(t time.Time) *Fake { return &Fake{now: t} }

// Now returns the fake time.
func (f *Fake) Now() time.Time {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.now
}

// Advance moves the fake time forward by d.
func (f *Fake) Advance(d time.Duration) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.now = f.now.Add(d)
}
