// Package timers is implementing setInterval setTimeout and co.
package timers

import (
	"time"

	"github.com/mstoykov/k6-taskqueue-lib/taskqueue"

	"github.com/dop251/goja"
	"go.k6.io/k6/js/modules"
)

// RootModule is the global module instance that will create module
// instances for each VU.
type RootModule struct{}

// Timers represents an instance of the timers module.
type Timers struct {
	vu modules.VU

	timerIDCounter uint64

	timers map[uint64]time.Time
	// Maybe in the future if this moves to core it will be expanded to have multiple queues
	queue *timerQueue

	// this used predominantly to get around very unlikely race conditions as we are adding stuff to the event loop
	// from outside of it on multitple timers. And it is easier to just use this then redo half the work it does
	// to make that safe
	taskQueue *taskqueue.TaskQueue
}

var (
	_ modules.Module   = &RootModule{}
	_ modules.Instance = &Timers{}
)

// New returns a pointer to a new RootModule instance.
func New() *RootModule {
	return &RootModule{}
}

// NewModuleInstance implements the modules.Module interface to return
// a new instance for each VU.
func (*RootModule) NewModuleInstance(vu modules.VU) modules.Instance {
	return &Timers{
		vu:     vu,
		timers: make(map[uint64]time.Time),
		queue:  new(timerQueue),
	}
}

// Exports returns the exports of the k6 module.
func (e *Timers) Exports() modules.Exports {
	return modules.Exports{
		Named: map[string]interface{}{
			"setTimeout":    e.setTimeout,
			"clearTimeout":  e.clearTimeout,
			"setInterval":   e.setInterval,
			"clearInterval": e.clearInterval,
		},
	}
}

func (e *Timers) nextID() uint64 {
	e.timerIDCounter++
	return e.timerIDCounter
}

func (e *Timers) call(callback goja.Callable, args []goja.Value) error {
	// TODO: investigate, not sure GlobalObject() is always the correct value for `this`?
	_, err := callback(e.vu.Runtime().GlobalObject(), args...)
	return err
}

func (e *Timers) setTimeout(callback goja.Callable, delay float64, args ...goja.Value) uint64 {
	id := e.nextID()
	e.timerInitialization(callback, delay, args, false, id)
	return id
}

func (e *Timers) clearTimeout(id uint64) {
	_, exists := e.timers[id]
	if !exists {
		return
	}
	delete(e.timers, id)

	e.queue.remove(id)
	e.freeEventLoopIfPossible()
}

func (e *Timers) freeEventLoopIfPossible() {
	if e.queue.length() == 0 && e.taskQueue != nil {
		e.taskQueue.Close()
		e.taskQueue = nil
	}
}

func (e *Timers) setInterval(callback goja.Callable, delay float64, args ...goja.Value) uint64 {
	id := e.nextID()
	e.timerInitialization(callback, delay, args, true, id)
	return id
}

func (e *Timers) clearInterval(id uint64) {
	e.clearTimeout(id)
}

// https://html.spec.whatwg.org/multipage/timers-and-user-prompts.html#timer-initialisation-steps
// NOTE: previousId from the specification is always send and it is basically id
func (e *Timers) timerInitialization(
	callback goja.Callable, timeout float64, args []goja.Value, repeat bool, id uint64,
) {
	// skip all the nesting stuff as we do not care about them
	if timeout < 0 {
		timeout = 0
	}

	task := func() error {
		// Specification 8.1: If id does not exist in global's map of active timers, then abort these steps.
		if _, exist := e.timers[id]; !exist {
			return nil
		}

		err := e.call(callback, args)

		if _, exist := e.timers[id]; !exist { // 8.4
			return err
		}

		if repeat {
			e.timerInitialization(callback, timeout, args, repeat, id)
		} else {
			delete(e.timers, id)
		}

		return err
	}

	e.runAfterTimeout(timeout, task, id)
}

// https://html.spec.whatwg.org/multipage/timers-and-user-prompts.html#run-steps-after-a-timeout
// Notes:
// orderingId is not really used in this case
// id is also required for us unlike how it is defined. Maybe in the future if this moves to core it will be expanded
func (e *Timers) runAfterTimeout(timeout float64, task func() error, id uint64) {
	delay := time.Duration(timeout * float64(time.Millisecond))
	triggerTime := time.Now().Add(delay)
	e.timers[id] = triggerTime

	// as we have only one orderingId we have one queue
	index := e.queue.add(&timer{
		id:          id,
		task:        task,
		nextTrigger: triggerTime,
	})

	if index != 0 {
		return // not a timer at the very beginning
	}

	e.setupTaskTimeout()
}

func (e *Timers) runFirstTask() error {
	t := e.queue.pop()
	if t == nil {
		return nil // everything was cleared
	}

	err := t.task()

	if e.queue.length() > 0 {
		e.setupTaskTimeout()
	} else {
		e.freeEventLoopIfPossible()
	}

	return err
}

func (e *Timers) setupTaskTimeout() {
	e.queue.stopTimer()
	delay := -time.Since(e.timers[e.queue.first().id])
	if e.taskQueue == nil {
		e.taskQueue = taskqueue.New(e.vu.RegisterCallback)
	}
	q := e.taskQueue
	e.queue.head = time.AfterFunc(delay, func() {
		q.Queue(e.runFirstTask)
	})
}

// this is just a small struct to keep the internals of a timer
type timer struct {
	id          uint64
	nextTrigger time.Time
	task        func() error
}

// this is just a list of timers that should be ordered once after the other
// this mostly just has methods to work on the slice
type timerQueue struct {
	queue []*timer
	head  *time.Timer
}

func (tq *timerQueue) add(t *timer) int {
	var i int
	// don't use range as we want to index to go over one if it needs to go to the end
	for ; i < len(tq.queue); i++ {
		if tq.queue[i].nextTrigger.After(t.nextTrigger) {
			break
		}
	}

	tq.queue = append(tq.queue, nil)
	copy(tq.queue[i+1:], tq.queue[i:])
	tq.queue[i] = t
	return i
}

func (tq *timerQueue) stopTimer() {
	if tq.head != nil && tq.head.Stop() { // we have a timer and we stopped it before it was over.
		select {
		case <-tq.head.C:
		default:
		}
	}
}

func (tq *timerQueue) remove(id uint64) {
	i := tq.findIndex(id)
	if i == -1 {
		return
	}

	tq.queue = append(tq.queue[:i], tq.queue[i+1:]...)
}

func (tq *timerQueue) findIndex(id uint64) int {
	for i, timer := range tq.queue {
		if id == timer.id {
			return i
		}
	}
	return -1
}

func (tq *timerQueue) pop() *timer {
	length := len(tq.queue)
	if length == 0 {
		return nil
	}
	t := tq.queue[0]
	copy(tq.queue, tq.queue[1:])
	tq.queue = tq.queue[:length-1]
	return t
}

func (tq *timerQueue) length() int {
	return len(tq.queue)
}

func (tq *timerQueue) first() *timer {
	if tq.length() == 0 {
		return nil
	}
	return tq.queue[0]
}
