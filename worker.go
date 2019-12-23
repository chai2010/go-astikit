package astikit

import (
	"context"
	"os"
	"os/signal"
	"sync"
)

// Worker represents an object capable of blocking, handling signals and stopping
type Worker struct {
	cancel context.CancelFunc
	ctx    context.Context
	l      Logger
	os, ow sync.Once
	wg     *sync.WaitGroup
}

// NewWorker builds a new worker
func NewWorker(l Logger) (w *Worker) {
	w = &Worker{
		l:  newNopLogger(),
		wg: &sync.WaitGroup{},
	}
	w.ctx, w.cancel = context.WithCancel(context.Background())
	w.wg.Add(1)
	if l != nil {
		w.l = l
	}
	w.l.Info("astikit: starting worker...")
	return
}

// HandleSignals handles signals
func (w *Worker) HandleSignals(hs ...SignalHandler) {
	// Add default handler
	hs = append([]SignalHandler{TermSignalHandler(w.Stop)}, hs...)

	// Notify
	ch := make(chan os.Signal, 1)
	signal.Notify(ch)

	// Execute in a task
	w.NewTask().Do(func() {
		for {
			select {
			case s := <-ch:
				// Log
				w.l.Debugf("astikit: received signal %s", s)

				// Loop through handlers
				for _, h := range hs {
					h(s)
				}

				// Return
				if isTermSignal(s) {
					return
				}
			case <-w.Context().Done():
				return
			}
		}
	})
}

// Stop stops the Worker
func (w *Worker) Stop() {
	w.os.Do(func() {
		w.l.Info("astikit: stopping worker...")
		w.cancel()
		w.wg.Done()
	})
}

// Wait is a blocking pattern
func (w *Worker) Wait() {
	w.ow.Do(func() {
		w.l.Info("astikit: worker is now waiting...")
		w.wg.Wait()
	})
}

// NewTask creates a new task
func (w *Worker) NewTask() *Task {
	return newTask(w.wg)
}

// Context returns the worker's context
func (w *Worker) Context() context.Context {
	return w.ctx
}

// Logger returns the worker's logger
func (w *Worker) Logger() Logger {
	return w.l
}

// Task represents a task
type Task struct {
	od, ow  sync.Once
	wg, pwg *sync.WaitGroup
}

func newTask(parentWg *sync.WaitGroup) (t *Task) {
	t = &Task{
		wg:  &sync.WaitGroup{},
		pwg: parentWg,
	}
	t.pwg.Add(1)
	return
}

// TaskFunc represents a function that can create a new task
type TaskFunc func() *Task

// NewSubTask creates a new sub task
func (t *Task) NewSubTask() *Task {
	return newTask(t.wg)
}

// Do executes the task
func (t *Task) Do(f func()) {
	go func() {
		// Make sure to mark the task as done
		defer t.Done()

		// Custom
		f()
	}()
}

// Done indicates the task is done
func (t *Task) Done() {
	t.od.Do(func() {
		t.pwg.Done()
	})
}

// Wait waits for the task to be finished
func (t *Task) Wait() {
	t.ow.Do(func() {
		t.wg.Wait()
	})
}