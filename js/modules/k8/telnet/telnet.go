package telnet

import (
	"context"
	"errors"
	"strings"

	"github.com/dop251/goja"
	"github.com/runner-mei/goutils/shell/harness"
	"github.com/runner-mei/k8/js/common"
	"github.com/runner-mei/k8/lib"
	"github.com/runner-mei/k8/js/modules/k8/ssh"
)

// ErrTelnetInInitContext is returned when telnet are using in the init context
var ErrTelnetInInitContext = common.NewInitContextError("using telnet in the init context is not supported")

type Telnet struct{}


func New() *Telnet {
	return &Telnet{}
}

func (s *Telnet) Do(ctx context.Context, params goja.Value, callable goja.Value) (goja.Value, error) {
	rt := common.GetRuntime(ctx)
	state := lib.GetState(ctx)
	if state == nil {
		return nil, ErrTelnetInInitContext
	}

	// Get the callable (required)
	callFn, isFunc := goja.AssertFunction(callable)
	if !isFunc {
		return nil, errors.New("last argument to Telnet.connect must be a function")
	}

	var telnetParams harness.TelnetParam
	// Parse the optional second argument (params)
	if !goja.IsUndefined(params) && !goja.IsNull(params) {
		params := params.ToObject(rt)
		for _, k := range params.Keys() {
			switch k {
			case "telnet.address":
				telnetParams.Address = params.Get(k).String()
			case "telnet.port":
				telnetParams.Port = params.Get(k).String()
			case "telnet.user_quest", "telnet.username_quest":
				telnetParams.UsernameQuest = params.Get(k).String()
			case "telnet.username":
				telnetParams.Username = params.Get(k).String()
			case "telnet.password_quest":
				telnetParams.PasswordQuest = params.Get(k).String()
			case "telnet.password":
				telnetParams.Password = params.Get(k).String()
			case "telnet.prompt":
				telnetParams.Prompt = params.Get(k).String()
			case "telnet.enable_command":
				telnetParams.EnableCommand = params.Get(k).String()
			case "telnet.enable_password_quest":
				telnetParams.EnablePasswordQuest = params.Get(k).String()
			case "telnet.enable_password":
				telnetParams.EnablePassword = params.Get(k).String()
			case "telnet.EnablePrompt":
				telnetParams.EnablePrompt = params.Get(k).String()
			case "telnet.use_crlf":
				telnetParams.UseCRLF = strings.ToLower(params.Get(k).String()) == "true"
			}
		}
	}

	if telnetParams.Address == "" {
		return nil, errors.New("address is missing")
	}

	shell, err := s.acquire(ctx, &telnetParams)
	if err != nil {
		return nil, err
	}
	defer s.release(ctx, shell)

	return callFn(goja.Undefined(), rt.ToValue(shell.BindObject))
}

func (*Telnet) acquire(ctx context.Context, telnetParams *harness.TelnetParam) (ssh.Shell, error) {
	shell := ssh.Shell{
		Ctx: ctx,
		Conn: &harness.Shell{
			TelnetParams: telnetParams,
		},
	}

	err := shell.Conn.Connect(ctx, "telnet")
	if err == nil {
		shell.BindObject = map[string]interface{}{
			"readPrompt": shell.ReadPrompt,
			"write":      shell.Write,
			"sendln":     shell.Sendln,
			"exec":       shell.Exec,
			"runScript":  shell.RunScript,
		}
	}
	return shell, err
}

func (*Telnet) release(ctx context.Context, sh ssh.Shell) {
	sh.Close()
}
