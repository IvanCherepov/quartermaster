// Copyright 2017 The quartermaster Authors
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

package tests

import (
	"bytes"
	"encoding/json"
	"io"
	"io/ioutil"
)

func ObjectToJSONBody(object interface{}) (io.ReadCloser, error) {
	j, err := json.Marshal(object)
	if err != nil {
		return nil, err
	}
	return ioutil.NopCloser(bytes.NewReader(j)), nil
}
