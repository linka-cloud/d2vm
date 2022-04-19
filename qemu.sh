#!/usr/bin/env bash

# Copyright 2021 Linka Cloud  All rights reserved.
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#   http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.

IMG=${1:-disk0.qcow2}

ARGS=""

if [[ $OSTYPE == 'darwin'* ]]; then
  ARGS="-M accel=hvf"
else
  ARGS="-use-kvm"
fi

qemu-system-x86_64 -drive file=$IMG,index=0,media=disk,format=qcow2 -m 4096 -cpu host -nographic ${ARGS} ${@: 2}
