/*
 *
 * k6 - a next-generation load testing tool
 * Copyright (C) 2017 Load Impact
 *
 * This program is free software: you can redistribute it and/or modify
 * it under the terms of the GNU Affero General Public License as
 * published by the Free Software Foundation, either version 3 of the
 * License, or (at your option) any later version.
 *
 * This program is distributed in the hope that it will be useful,
 * but WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
 * GNU Affero General Public License for more details.
 *
 * You should have received a copy of the GNU Affero General Public License
 * along with this program.  If not, see <http://www.gnu.org/licenses/>.
 *
 */
package ssh

import (
	"cn/com/hengwei/sim/sshd"
	"context"
	"testing"

	"github.com/dop251/goja"
	"github.com/runner-mei/k8/js/common"
	"github.com/runner-mei/k8/lib"

	// "github.com/runner-mei/k8/lib/metrics"

	// "github.com/runner-mei/k8/stats"
	"github.com/stretchr/testify/assert"
	"github.com/runner-mei/goutils/shell/harness"
)

func TestSession(t *testing.T) {
	options := &sshd.Options{}
	options.AddUserPassword("abc", "123")

	options.WithEnable("ABC>", "enable", "password:", "<<none>>", "", "abc#", sshd.Echo)
	//options.WithNoEnable("ABC>", sshd.Echo)

	listener, err := sshd.StartServer(":", options)
	if err != nil {
		t.Error(err)
		return
	}
	defer listener.Close()

	port := listener.Port()

	rt := goja.New()
	rt.SetFieldNameMapper(common.FieldNameMapper{})
	// samples := make(chan stats.SampleContainer, 1000)
	state := &lib.State{}

	ctx := context.Background()
	ctx = lib.WithState(ctx, state)
	ctx = common.WithRuntime(ctx, rt)

	rt.Set("ssh", common.Bind(rt, New(), &common.BridgeContext{CtxPtr: &ctx}))

	t.Run("connect_ssh", func(t *testing.T) {
		res, err := common.RunString(rt, `
		ssh.do({
			"ssh.address": "127.0.0.1",
			"ssh.port": "`+port+`",
			"ssh.username": "abc",
			"ssh.password": "123",
			"ssh.enable_command": "enable",
			"ssh.enable_password": "<<none>>",
			"ssh.use_crlf": "true",
		}, function(sh) {
			return sh.exec('echo ssh_test_ok')
		});
		`)
		assert.NoError(t, err)
		assert.NotNil(t, res)

		if res != nil {
			o := res.Export()

			eres, ok := o.(*harness.ExecuteResult)
			if !ok {
				t.Errorf("%T %v", o, o)
				return 
			}

			if  eres.Incomming != "print ssh_test_ok\r\nabc#" {
				t.Logf("%#v", *eres)
			}
		}
	})

}
