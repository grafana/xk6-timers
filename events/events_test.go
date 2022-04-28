package events

import (
	"context"
	"testing"
	"time"

	"github.com/dop251/goja"
	"github.com/stretchr/testify/require"
	"go.k6.io/k6/js/common"
	"go.k6.io/k6/js/eventloop"
	"go.k6.io/k6/js/modulestest"
)

func TestSetTimeout(t *testing.T) {
	t.Parallel()
	rt := goja.New()
	vu := &modulestest.VU{
		RuntimeField: rt,
		InitEnvField: &common.InitEnvironment{},
		CtxField:     context.Background(),
		StateField:   nil,
	}

	m, ok := New().NewModuleInstance(vu).(*Events)
	require.True(t, ok)
	var log []string
	require.NoError(t, rt.Set("events", m.Exports().Named))
	require.NoError(t, rt.Set("print", func(s string) { log = append(log, s) }))
	loop := eventloop.New(vu)
	vu.RegisterCallbackField = loop.RegisterCallback

	err := loop.Start(func() error {
		_, err := vu.Runtime().RunString(`
      events.setTimeout(()=> {
        print("in setTimeout")
      })
      print("outside setTimeout")
      `)
		return err
	})
	require.NoError(t, err)
	require.Equal(t, []string{"outside setTimeout", "in setTimeout"}, log)
}

func TestSetInterval(t *testing.T) {
	t.Parallel()
	rt := goja.New()
	vu := &modulestest.VU{
		RuntimeField: rt,
		InitEnvField: &common.InitEnvironment{},
		CtxField:     context.Background(),
		StateField:   nil,
	}

	m, ok := New().NewModuleInstance(vu).(*Events)
	require.True(t, ok)
	var log []string
	require.NoError(t, rt.Set("events", m.Exports().Named))
	require.NoError(t, rt.Set("print", func(s string) { log = append(log, s) }))
	require.NoError(t, rt.Set("sleep10", func() { time.Sleep(10 * time.Millisecond) }))
	loop := eventloop.New(vu)
	vu.RegisterCallbackField = loop.RegisterCallback

	err := loop.Start(func() error {
		_, err := vu.Runtime().RunString(`
      var i = 0;
      let s = events.setInterval(()=> {
        sleep10();
        if (i>1) {
          print("in setInterval");
          events.clearInterval(s);
        }
        i++;
      }, 1);
      print("outside setInterval")
      `)
		return err
	})
	require.NoError(t, err)
	require.True(t, len(log) > 2)
	require.Equal(t, "outside setInterval", log[0])
	for i, l := range log[1:] {
		require.Equal(t, "in setInterval", l, i)
	}
}
