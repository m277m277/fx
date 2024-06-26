// Copyright (c) 2019-2021 Uber Technologies, Inc.
//
// Permission is hereby granted, free of charge, to any person obtaining a copy
// of this software and associated documentation files (the "Software"), to deal
// in the Software without restriction, including without limitation the rights
// to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
// copies of the Software, and to permit persons to whom the Software is
// furnished to do so, subject to the following conditions:
//
// The above copyright notice and this permission notice shall be included in
// all copies or substantial portions of the Software.
//
// THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
// IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
// FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
// AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
// LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
// OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN
// THE SOFTWARE.

package fx

import (
	"context"
	"errors"
	"fmt"
	"os"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/fx/fxevent"
	"go.uber.org/fx/internal/fxclock"
	"go.uber.org/fx/internal/fxlog"
	"go.uber.org/fx/internal/fxreflect"
)

func TestAppRun(t *testing.T) {
	t.Parallel()

	spy := new(fxlog.Spy)
	app := New(
		WithLogger(func() fxevent.Logger { return spy }),
	)
	done := make(chan ShutdownSignal)

	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		app.run(func() <-chan ShutdownSignal { return done })
	}()

	done <- ShutdownSignal{Signal: _sigINT}
	wg.Wait()

	assert.Equal(t, []string{
		"Provided",
		"Provided",
		"Provided",
		"LoggerInitialized",
		"Started",
		"Stopping",
		"Stopped",
	}, spy.EventTypes())
}

// TestValidateString verifies private option. Public options are tested in app_test.go.
func TestValidateString(t *testing.T) {
	t.Parallel()

	stringer, ok := validate(true).(fmt.Stringer)
	require.True(t, ok, "option must implement stringer")
	assert.Equal(t, "fx.validate(true)", stringer.String())
}

// WithExit is an internal option available only to tests defined in this
// package. It changes how os.Exit behaves for the application.
func WithExit(f func(int)) Option {
	return withExitOption(f)
}

type withExitOption func(int)

func (o withExitOption) String() string {
	return fmt.Sprintf("WithExit(%v)", fxreflect.FuncName(o))
}

func (o withExitOption) apply(m *module) {
	m.app.osExit = o
}

// WithClock specifies how Fx accesses time operations.
//
// This is an internal option available only to tests defined in this package.
func WithClock(clock fxclock.Clock) Option {
	return withClockOption{clock}
}

type withClockOption struct{ clock fxclock.Clock }

func (o withClockOption) apply(m *module) {
	m.app.clock = o.clock
}

func (o withClockOption) String() string {
	return fmt.Sprintf("WithClock(%v)", o.clock)
}

func TestAnnotationError(t *testing.T) {
	wantErr := errors.New("want error")
	err := &annotationError{
		err: wantErr,
	}
	require.Error(t, err)
	assert.ErrorIs(t, err, wantErr)
	assert.Contains(t, err.Error(), wantErr.Error())
}

// TestStartDoesNotRegisterSignals verifies that signal.Notify is not called
// when a user starts an app. signal.Notify should only be called when the
// .Wait/.Done are called. Note that app.Run calls .Wait() implicitly.
func TestStartDoesNotRegisterSignals(t *testing.T) {
	app := New()
	calledNotify := false

	// Mock notify function to spy when this is called.
	app.receivers.notify = func(c chan<- os.Signal, sig ...os.Signal) {
		calledNotify = true
	}
	app.receivers.stopNotify = func(c chan<- os.Signal) {}

	app.Start(context.Background())
	defer app.Stop(context.Background())
	assert.False(t, calledNotify, "notify should not be called when app starts")

	_ = app.Wait() // User signals intent have fx listen for signals. This should call notify
	assert.True(t, calledNotify, "notify should be called after Wait")
}
