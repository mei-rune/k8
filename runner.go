package k8

import (
	"context"
	"net/http/cookiejar"
	"time"

	"github.com/dop251/goja"
	"github.com/runner-mei/gojs"
	"github.com/runner-mei/gojs/lib"
)

type Method struct {
	Meta   map[string]interface{}
	Method goja.Callable
}

// A Runner is a self-contained instance of a Bundle.
type Runner struct {
	NoCookiesReset *bool
	Runtime        *gojs.Runtime
	Default        goja.Callable
	Methods        map[string]Method
}

// Runs an exported function in its own temporary VU, optionally with an argument. Execution is
// interrupted if the context expires. No error is returned if the part does not exist.
func (runner *Runner) RunPart(ctx context.Context, name string, arg interface{}) (interface{}, error) {
	exp := runner.Runtime.Get("exports").ToObject(runner.Runtime.Runtime)
	if exp == nil {
		return goja.Undefined(), nil
	}
	fn, ok := goja.AssertFunction(exp.Get(name))
	if !ok {
		return goja.Undefined(), nil
	}

	return runner.RunFn(ctx, name, fn, runner.Runtime.ToValue(arg))
}

func (runner *Runner) RunDefaultMethod(ctx context.Context, arg interface{}) (interface{}, error) {
	if runner.Default == nil {
		return nil, ErrMethodMissing
	}
	return runner.RunFn(ctx /*group, */, "default", runner.Default, runner.Runtime.ToValue(arg))
}

func (runner *Runner) RunMethod(ctx context.Context, name string, arg interface{}) (interface{}, error) {
	fn, ok := runner.Methods[name]
	if !ok {
		return nil, ErrMethodMissing
	}
	return runner.RunFn(ctx /*group, */, name, fn.Method, runner.Runtime.ToValue(arg))
}

func (runner *Runner) RunFn(
	ctx context.Context, fnname string, fn goja.Callable, args ...goja.Value,
) (interface{}, error) {
	if runner.NoCookiesReset == nil || !*runner.NoCookiesReset {
		cookieJar, err := cookiejar.New(nil)
		if err != nil {
			return nil, err
		}

		old := lib.GetState(ctx)
		if old != nil {
			state := &lib.State{}
			*state = *old
			state.CookieJar = cookieJar
			ctx = lib.WithState(ctx, state)
		}
	}
	runner.Runtime.SetContext(ctx)
	v, err := fn(goja.Undefined(), args...) // Actually run the JS script
	if err != nil {
		// deadline is reached so we have timeouted but this might've not been registered correctly
		if deadline, ok := ctx.Deadline(); ok && time.Now().After(deadline) {
			// we could have an error that is not errInterrupt in which case we should return it instead
			if err, ok := err.(*goja.InterruptedError); ok && v != nil && err.Value() != errInterrupt {
				return v, err
			}
			// otherwise we have timeouted
			return v, lib.NewTimeoutError(fnname, 0)
		}

		return nil, err
	}
	return v.Export(), nil
}
