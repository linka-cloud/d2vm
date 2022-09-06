// Copyright 2022 Linka Cloud  All rights reserved.
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

type ConvertOption func(o *convertOptions)

type convertOptions struct {
	size           int64
	password       string
	output         string
	cmdLineExtra   string
	networkManager NetworkManager
	raw            bool
}

func WithSize(size int64) ConvertOption {
	return func(o *convertOptions) {
		o.size = size
	}
}

func WithPassword(password string) ConvertOption {
	return func(o *convertOptions) {
		o.password = password
	}
}

func WithOutput(output string) ConvertOption {
	return func(o *convertOptions) {
		o.output = output
	}
}

func WithCmdLineExtra(cmdLineExtra string) ConvertOption {
	return func(o *convertOptions) {
		o.cmdLineExtra = cmdLineExtra
	}
}

func WithNetworkManager(networkManager NetworkManager) ConvertOption {
	return func(o *convertOptions) {
		o.networkManager = networkManager
	}
}

func WithRaw(raw bool) ConvertOption {
	return func(o *convertOptions) {
		o.raw = raw
	}
}
