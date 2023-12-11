// Package timers is implementing setInterval setTimeout and co.
package timers

import (
	"sync/atomic"
	"time"

	"github.com/dop251/goja"
	"go.k6.io/k6/js/modules"
)

// RootModule is the global module instance that will create module
// instances for each VU.
type RootModule struct{}

// Timers represents an instance of the timers module.
type Timers struct {
	vu modules.VU

	timerStopCounter uint32

	timers map[int]time.Time
	// it is just a list of the id in their time.Time order.
	// it is used to get timers fire in sequence.
	// not anything more then a slice as it is unlikely it will have too many ids to begin with.
	timersQueue []int
	tasks       []func() error
	headTimer   *time.Timer

	runOnLoop func(func() error)
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
		timers: make(map[int]time.Time),
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

func (e *Timers) nextID() uint32 {
	return atomic.AddUint32(&e.timerStopCounter, 1)
}

func (e *Timers) call(callback goja.Callable, args []goja.Value) error {
	// TODO: investigate, not sure GlobalObject() is always the correct value for `this`?
	_, err := callback(e.vu.Runtime().GlobalObject(), args...)
	return err
}

func (e *Timers) setTimeout(callback goja.Callable, delay float64, args ...goja.Value) uint32 {
	id := e.nextID()
	e.timerInitialization(callback, delay, args, false, int(id))
	return id
}

func (e *Timers) clearTimeout(id uint32) {
	_, exists := e.timers[int(id)]
	if !exists {
		return
	}
	delete(e.timers, int(id))
	var i, otherID int
	var found bool
	for i, otherID = range e.timersQueue {
		if id == uint32(otherID) {
			found = true
			break
		}
	}
	if !found {
		return
	}

	e.timersQueue = append(e.timersQueue[:i], e.timersQueue[i+1:]...)
	e.tasks = append(e.tasks[:i], e.tasks[i+1:]...)
	// no need to touch the timer - if it was for this it will just do nothing and if it wasn't it will just skip it
}

func (e *Timers) setInterval(callback goja.Callable, delay float64, args ...goja.Value) uint32 {
	id := e.nextID()
	e.timerInitialization(callback, delay, args, true, int(id))
	return id
}

func (e *Timers) clearInterval(id uint32) {
	e.clearTimeout(id)
}

// https://html.spec.whatwg.org/multipage/timers-and-user-prompts.html#timer-initialisation-steps
// NOTE: previousId from the specification is always send and it is basically id
func (e *Timers) timerInitialization(callback goja.Callable, timeout float64, args []goja.Value, repeat bool, id int) {
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
func (e *Timers) runAfterTimeout(timeout float64, task func() error, id int) {
	// TODO figure out a better name
	delay := time.Duration(timeout * float64(time.Millisecond))
	timer := time.Now().Add(delay)
	e.timers[id] = timer

	// as we have only one orderingId we have one queue
	// TODO add queue type and a map of queues when/if we have more then one orderingId
	var index int
	// don't use range as we want to index to go over one if it needs to go to the end
	for index = 0; index < len(e.timersQueue); index++ {
		otherTimer := e.timers[e.timersQueue[index]]
		if otherTimer.After(timer) {
			break
		}
	}

	e.timersQueue = append(e.timersQueue, 0)
	copy(e.timersQueue[index+1:], e.timersQueue[index:])
	e.timersQueue[index] = id

	e.tasks = append(e.tasks, nil)
	copy(e.tasks[index+1:], e.tasks[index:])
	e.tasks[index] = task

	if index != 0 {
		// we are not the earliers in the queue so we can stop here
		return
	}
	e.setupTaskTimeout()
}

func (e *Timers) runFirstTask() error {
	e.runOnLoop = nil
	tasksLen := len(e.tasks)
	if tasksLen == 0 {
		return nil // everything was cleared
	}

	task := e.tasks[0]
	copy(e.tasks, e.tasks[1:])
	e.tasks = e.tasks[:tasksLen-1]

	copy(e.timersQueue, e.timersQueue[1:])
	e.timersQueue = e.timersQueue[:tasksLen-1]

	err := task()

	if len(e.timersQueue) > 0 {
		e.setupTaskTimeout()
	}
	return err
}

func (e *Timers) setupTaskTimeout() {
	if e.headTimer != nil {
		e.headTimer.Stop()
		select {
		case <-e.headTimer.C:
		default:
		}
	}
	delay := -time.Since(e.timers[e.timersQueue[0]])
	if e.runOnLoop == nil {
		e.runOnLoop = e.vu.RegisterCallback()
	}
	e.headTimer = time.AfterFunc(delay, func() {
		e.runOnLoop(e.runFirstTask)
	})
}
