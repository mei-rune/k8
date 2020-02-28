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

package modules

import (
	"github.com/runner-mei/k8/js/modules/k8"
	"github.com/runner-mei/k8/js/modules/k8/crypto"
	"github.com/runner-mei/k8/js/modules/k8/crypto/x509"
	"github.com/runner-mei/k8/js/modules/k8/encoding"
	"github.com/runner-mei/k8/js/modules/k8/html"
	"github.com/runner-mei/k8/js/modules/k8/http"
	"github.com/runner-mei/k8/js/modules/k8/ws"
)

// Index of module implementations.
var Index = map[string]interface{}{
	"k8":             k8.New(),
	"k8/crypto":      crypto.New(),
	"k8/crypto/x509": x509.New(),
	"k8/encoding":    encoding.New(),
	"k8/http":        http.New(),
	"k8/html":        html.New(),
	"k8/ws":          ws.New(),
}
