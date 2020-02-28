package js

import (
	"context"
	"crypto/tls"
	"net"
	"net/http"
	"net/http/cookiejar"
	"time"

	"github.com/dop251/goja"
	"github.com/oxtoacart/bpool"
	"github.com/runner-mei/k8/js/common"
	"github.com/runner-mei/k8/lib"
	"github.com/runner-mei/k8/lib/netext"
	"github.com/runner-mei/log"
	"github.com/viki-org/dnscache"
	"golang.org/x/net/http2"
)

type Method struct {
	meta   map[string]interface{}
	method goja.Callable
}

// A Runner is a self-contained instance of a Bundle.
type Runner struct {
	NoCookiesReset *bool
	Runtime        *goja.Runtime
	Context        *context.Context
	Default        goja.Callable
	Methods        map[string]Method
}

// Runs an exported function in its own temporary VU, optionally with an argument. Execution is
// interrupted if the context expires. No error is returned if the part does not exist.
func (bi *Runner) RunPart(ctx context.Context, name string, arg interface{}) (interface{}, error) {
	exp := bi.Runtime.Get("exports").ToObject(bi.Runtime)
	if exp == nil {
		return goja.Undefined(), nil
	}
	fn, ok := goja.AssertFunction(exp.Get(name))
	if !ok {
		return goja.Undefined(), nil
	}

	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	go func() {
		<-ctx.Done()
		bi.Runtime.Interrupt(errInterrupt)
	}()

	v, err := bi.RunFn(ctx /*group, */, fn, bi.Runtime.ToValue(arg))

	// deadline is reached so we have timeouted but this might've not been registered correctly
	if deadline, ok := ctx.Deadline(); ok && time.Now().After(deadline) {
		// we could have an error that is not errInterrupt in which case we should return it instead
		if err, ok := err.(*goja.InterruptedError); ok && v != nil && err.Value() != errInterrupt {
			return v, err
		}
		// otherwise we have timeouted
		return v, lib.NewTimeoutError(name, 0)
	}
	if err != nil {
		return nil, err
	}
	return v.Export(), nil
}

func (bi *Runner) RunDefaultMethod(ctx context.Context, arg interface{}) (interface{}, error) {
	if bi.Default == nil {
		return nil, ErrMethodMissing
	}
	v, err := bi.RunFn(ctx /*group, */, bi.Default, bi.Runtime.ToValue(arg))
	if err != nil {
		// deadline is reached so we have timeouted but this might've not been registered correctly
		if deadline, ok := ctx.Deadline(); ok && time.Now().After(deadline) {
			// we could have an error that is not errInterrupt in which case we should return it instead
			if err, ok := err.(*goja.InterruptedError); ok && v != nil && err.Value() != errInterrupt {
				return v, err
			}
			// otherwise we have timeouted
			return v, lib.NewTimeoutError("default", 0)
		}

		return nil, err
	}
	return v.Export(), nil
}

func (bi *Runner) RunMethod(ctx context.Context, name string, arg interface{}) (interface{}, error) {
	fn, ok := bi.Methods[name]
	if !ok {
		return nil, ErrMethodMissing
	}
	v, err := bi.RunFn(ctx /*group, */, fn.method, bi.Runtime.ToValue(arg))
	if err != nil {
		// deadline is reached so we have timeouted but this might've not been registered correctly
		if deadline, ok := ctx.Deadline(); ok && time.Now().After(deadline) {
			// we could have an error that is not errInterrupt in which case we should return it instead
			if err, ok := err.(*goja.InterruptedError); ok && v != nil && err.Value() != errInterrupt {
				return v, err
			}
			// otherwise we have timeouted
			return v, lib.NewTimeoutError(name, 0)
		}

		return nil, err
	}
	return v.Export(), nil
}

func (bi *Runner) RunFn(
	ctx context.Context, fn goja.Callable, args ...goja.Value,
) (goja.Value, error) {
	if bi.NoCookiesReset == nil || !*bi.NoCookiesReset {
		cookieJar, err := cookiejar.New(nil)
		if err != nil {
			return goja.Undefined(), err
		}

		old := lib.GetState(ctx)
		if old != nil {
			state := &lib.State{}
			*state = *old
			state.CookieJar = cookieJar
			ctx = lib.WithState(ctx, state)
		}
	}

	ctx = common.WithRuntime(ctx, bi.Runtime)
	*bi.Context = ctx
	return fn(goja.Undefined(), args...) // Actually run the JS script
}

func NewState(logger log.Logger, opts lib.Options, baseDialer net.Dialer, resolver *dnscache.Resolver) (*lib.State, error) {
	var cipherSuites []uint16
	if opts.TLSCipherSuites != nil {
		cipherSuites = *opts.TLSCipherSuites
	}

	var tlsVersions lib.TLSVersions
	if opts.TLSVersion != nil {
		tlsVersions = *opts.TLSVersion
	}

	tlsAuth := opts.TLSAuth
	certs := make([]tls.Certificate, len(tlsAuth))
	nameToCert := make(map[string]*tls.Certificate)
	for i, auth := range tlsAuth {
		for _, name := range auth.Domains {
			cert, err := auth.Certificate()
			if err != nil {
				return nil, err
			}
			certs[i] = *cert
			nameToCert[name] = &certs[i]
		}
	}

	dialer := &netext.Dialer{
		Dialer:    baseDialer,
		Resolver:  resolver,
		Blacklist: opts.BlacklistIPs,
		Hosts:     opts.Hosts,
	}
	tlsConfig := &tls.Config{
		InsecureSkipVerify: opts.InsecureSkipTLSVerify.Bool,
		CipherSuites:       cipherSuites,
		MinVersion:         uint16(tlsVersions.Min),
		MaxVersion:         uint16(tlsVersions.Max),
		Certificates:       certs,
		NameToCertificate:  nameToCert,
		Renegotiation:      tls.RenegotiateFreelyAsClient,
	}
	transport := &http.Transport{
		Proxy:               http.ProxyFromEnvironment,
		TLSClientConfig:     tlsConfig,
		DialContext:         dialer.DialContext,
		DisableCompression:  true,
		DisableKeepAlives:   opts.NoConnectionReuse.Bool,
		MaxIdleConns:        int(opts.Batch.Int64),
		MaxIdleConnsPerHost: int(opts.BatchPerHost.Int64),
	}
	_ = http2.ConfigureTransport(transport)

	cookieJar, err := cookiejar.New(nil)
	if err != nil {
		return nil, err
	}

	return &lib.State{
		Logger:  logger,
		Options: opts,
		// Group:     group,
		Transport: transport,
		Dialer:    dialer,
		TLSConfig: tlsConfig,
		CookieJar: cookieJar,
		// RPSLimit:  u.Runner.RPSLimit,
		BPool: bpool.NewBufferPool(100),
		// Vu:        u.ID,
		// Samples: u.Samples,
		// Iteration: u.Iteration,
	}, nil
}
