/*
 *
 * k6 - a next-generation load testing tool
 * Copyright (C) 2016 Load Impact
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

package js

import (
	"context"
	"strconv"

	"github.com/dop251/goja"
	"github.com/runner-mei/log"
)

// console represents a JS console implemented as a log.Logger.
type console struct {
	logger log.Logger
}

// Creates a console with the standard log logger.
func newConsole(logger log.Logger) *console {
	return &console{logger: logger}
}

func newFileConsole(logger log.Logger, filename string) (*console, error) {
	return &console{logger: logger}, nil
}

func (c console) Log(ctx *context.Context, msg goja.Value, args ...goja.Value) {
	fields := make([]log.Field, 0, len(args))
	for i, arg := range args {
		fields = append(fields, log.Stringer(strconv.Itoa(i), arg))
	}
	c.logger.Info(msg.String(), fields...)
}

func (c console) Debug(ctx *context.Context, msg goja.Value, args ...goja.Value) {
	fields := make([]log.Field, 0, len(args))
	for i, arg := range args {
		fields = append(fields, log.Stringer(strconv.Itoa(i), arg))
	}
	c.logger.Debug(msg.String(), fields...)
}

func (c console) Info(ctx *context.Context, msg goja.Value, args ...goja.Value) {
	fields := make([]log.Field, 0, len(args))
	for i, arg := range args {
		fields = append(fields, log.Stringer(strconv.Itoa(i), arg))
	}
	c.logger.Info(msg.String(), fields...)
}

func (c console) Warn(ctx *context.Context, msg goja.Value, args ...goja.Value) {
	fields := make([]log.Field, 0, len(args))
	for i, arg := range args {
		fields = append(fields, log.Stringer(strconv.Itoa(i), arg))
	}
	c.logger.Warn(msg.String(), fields...)
}

func (c console) Error(ctx *context.Context, msg goja.Value, args ...goja.Value) {
	fields := make([]log.Field, 0, len(args))
	for i, arg := range args {
		fields = append(fields, log.Stringer(strconv.Itoa(i), arg))
	}
	c.logger.Error(msg.String(), fields...)
}
