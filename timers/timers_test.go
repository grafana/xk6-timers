package timers

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"go.k6.io/k6/js/modulestest"
)

func TestSetTimeout(t *testing.T) {
	t.Parallel()
	runtime := modulestest.NewRuntime(t)
	err := runtime.SetupModuleSystem(map[string]any{"k6/x/timers": New()}, nil, nil)
	require.NoError(t, err)

	rt := runtime.VU.Runtime()
	var log []string
	require.NoError(t, rt.Set("print", func(s string) { log = append(log, s) }))

	_, err = runtime.RunOnEventLoop(`
		let timers = require("k6/x/timers");
		timers.setTimeout(()=> {
			print("in setTimeout")
		})
		print("outside setTimeout")
	`)
	require.NoError(t, err)
	require.Equal(t, []string{"outside setTimeout", "in setTimeout"}, log)
}

func TestSetInterval(t *testing.T) {
	t.Parallel()
	runtime := modulestest.NewRuntime(t)
	err := runtime.SetupModuleSystem(map[string]any{"k6/x/timers": New()}, nil, nil)
	require.NoError(t, err)

	rt := runtime.VU.Runtime()
	var log []string
	require.NoError(t, rt.Set("print", func(s string) { log = append(log, s) }))
	require.NoError(t, rt.Set("sleep10", func() { time.Sleep(10 * time.Millisecond) }))

	_, err = runtime.RunOnEventLoop(`
		let timers = require("k6/x/timers");
		var i = 0;
		let s = timers.setInterval(()=> {
			sleep10();
			if (i>1) {
			  print("in setInterval");
			  timers.clearInterval(s);
			}
			i++;
		}, 1);
		print("outside setInterval")
	`)
	require.NoError(t, err)
	require.Greater(t, len(log), 2)
	require.Equal(t, "outside setInterval", log[0])
	for i, l := range log[1:] {
		require.Equal(t, "in setInterval", l, i)
	}
}
