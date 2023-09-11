// Copyright 2023 Linka Cloud  All rights reserved.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package d2vm

import (
	"context"
	"fmt"
)

var bootloaderProviders = map[string]BootloaderProvider{}

func RegisterBootloaderProvider(provider BootloaderProvider) {
	bootloaderProviders[provider.Name()] = provider
}

func BootloaderByName(name string) (BootloaderProvider, error) {
	if p, ok := bootloaderProviders[name]; ok {
		return p, nil
	}
	return nil, fmt.Errorf("bootloader provider %s not found", name)
}

type BootloaderProvider interface {
	New(c Config, r OSRelease) (Bootloader, error)
	Name() string
}

type Bootloader interface {
	Setup(ctx context.Context, dev, root, cmdline string) error
}
