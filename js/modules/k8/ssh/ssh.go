package ssh

import (
	"context"
	"errors"
	"strings"

	"github.com/dop251/goja"
	"github.com/runner-mei/goutils/shell/harness"
	"github.com/runner-mei/k8/js/common"
	"github.com/runner-mei/k8/lib"
)

// ErrSSHInInitContext is returned when ssh are using in the init context
var ErrSSHInInitContext = common.NewInitContextError("using ssh in the init context is not supported")

type SSH struct{}

type SSHResponse struct {
	Status int    `json:"status"`
	Error  string `json:"error"`
}

func New() *SSH {
	return &SSH{}
}

func (s *SSH) Do(ctx context.Context, params goja.Value, callable goja.Value) (goja.Value, error) {
	rt := common.GetRuntime(ctx)
	state := lib.GetState(ctx)
	if state == nil {
		return nil, ErrSSHInInitContext
	}

	// Get the callable (required)
	callFn, isFunc := goja.AssertFunction(callable)
	if !isFunc {
		return nil, errors.New("last argument to ssh.connect must be a function")
	}

	var sshParams harness.SSHParam
	// Parse the optional second argument (params)
	if !goja.IsUndefined(params) && !goja.IsNull(params) {
		params := params.ToObject(rt)
		for _, k := range params.Keys() {
			switch k {
			case "ssh.address":
				sshParams.Address = params.Get(k).String()
			case "ssh.port":
				sshParams.Port = params.Get(k).String()
			case "ssh.user_quest", "ssh.username_quest":
				sshParams.UsernameQuest = params.Get(k).String()
			case "ssh.username":
				sshParams.Username = params.Get(k).String()
			case "ssh.password_quest":
				sshParams.PasswordQuest = params.Get(k).String()
			case "ssh.password":
				sshParams.Password = params.Get(k).String()
			case "ssh.private_key":
				sshParams.PrivateKey = params.Get(k).String()
			case "ssh.prompt":
				sshParams.Prompt = params.Get(k).String()
			case "ssh.enable_command":
				sshParams.EnableCommand = params.Get(k).String()
			case "ssh.enable_password_quest":
				sshParams.EnablePasswordQuest = params.Get(k).String()
			case "ssh.enable_password":
				sshParams.EnablePassword = params.Get(k).String()
			case "ssh.EnablePrompt":
				sshParams.EnablePrompt = params.Get(k).String()
			case "ssh.use_external_ssh":
				sshParams.UseExternalSSH = strings.ToLower(params.Get(k).String()) == "true"
			case "ssh.use_crlf":
				sshParams.UseCRLF = strings.ToLower(params.Get(k).String()) == "true"
			}
		}
	}

	if sshParams.Address == "" {
		return nil, errors.New("address is missing")
	}

	if sshParams.Username == "" {
		return nil, errors.New("username is missing")
	}

	if sshParams.Password == "" {
		return nil, errors.New("password is missing")
	}

	shell, err := s.acquire(ctx, &sshParams)
	if err != nil {
		return nil, err
	}
	defer s.release(ctx, shell)

	return callFn(goja.Undefined(), rt.ToValue(shell.BindObject))
}

func (*SSH) acquire(ctx context.Context, sshParams *harness.SSHParam) (Shell, error) {
	shell := Shell{
		Ctx: ctx,
		Conn: &harness.Shell{
			SSHParams: sshParams,
		},
	}

	err := shell.Conn.Connect(ctx, "ssh")
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

func (*SSH) release(ctx context.Context, sh Shell) {
	sh.Close()
}

type Shell struct {
	Ctx        context.Context
	Conn       *harness.Shell
	BindObject map[string]interface{}
}

func (sh Shell) Close() error {
	if sh.Conn == nil {
		return nil
	}
	return sh.Conn.Close()
}

func (sh Shell) ReadPrompt(expected ...string) error {
	var prompts = make([][]byte, len(expected))
	for i := range expected {
		prompts[i] = []byte(expected[i])
	}
	return sh.Conn.ReadPrompt(sh.Ctx, prompts)
}

func (sh Shell) Write(s string) error {
	return sh.Conn.Write(sh.Ctx, []byte(s))
}

func (sh Shell) Sendln(s string) error {
	return sh.Conn.Sendln(sh.Ctx, []byte(s))
}

func (sh Shell) Exec(cmd string) (*harness.ExecuteResult, error) {
	return harness.Exec(sh.Ctx, sh.Conn, cmd)
}

func (sh Shell) RunScript(subScript string) ([]harness.ExecuteResult, error) {
	script, err := harness.ParseScript(strings.NewReader(subScript))
	if err != nil {
		return nil, err
	}

	return sh.Conn.RunScript(sh.Ctx, script)
}
