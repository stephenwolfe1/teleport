/*
Copyright 2022 Gravitational, Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package reversetunnel

import (
	"strings"
	"sync"

	"github.com/google/go-cmp/cmp"
)

func NewConnectedProxies() *ConnectedProxies {
	return &ConnectedProxies{
		change: make(chan struct{}),
	}
}

type ConnectedProxies struct {
	ids    []string
	change chan struct{}
	mu     sync.RWMutex
}

func (p *ConnectedProxies) ProxyIDs() []string {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.ids
}

func (p *ConnectedProxies) WaitForChange() <-chan struct{} {
	return p.change
}

func (p *ConnectedProxies) updateProxyIDs(ids []string) {
	p.mu.Lock()
	defer p.mu.Unlock()

	if cmp.Equal(p.ids, ids) {
		return
	}

	p.ids = ids

	go func() {
		select {
		case p.change <- struct{}{}:
		default:
		}
	}()
}

func getIDFromPrincipals(principals []string) (string, bool) {
	if len(principals) == 0 {
		return "", false
	}

	// ID will always be the first principal.
	id := principals[0]

	// Return the uuid from the format "<uuid>.<cluster-name>".
	if split := strings.Split(id, "."); len(split) > 1 {
		id = split[0]
	}

	return id, true
}
