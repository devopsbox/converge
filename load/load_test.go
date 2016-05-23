// Copyright © 2016 Asteris, LLC
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

package load_test

import (
	"fmt"
	"os"
	"path"
	"testing"

	"github.com/asteris-llc/converge/load"
	"github.com/stretchr/testify/assert"
)

var samplesDir string

func init() {
	wd, _ := os.Getwd()
	samplesDir = path.Join(wd, "..", "samples")
}

func TestLoadBasic(t *testing.T) {
	_, err := load.Load(path.Join(samplesDir, "basic.hcl"))
	assert.NoError(t, err)
}

func TestLoadNotExist(t *testing.T) {
	badPath := path.Join(samplesDir, "doesNotExist.hcl")
	_, err := load.Load(badPath)
	if assert.Error(t, err) {
		assert.EqualError(t, err, fmt.Sprintf("Not found: %q using protocol \"file\"", badPath))
	}
}

func TestLoadFileModule(t *testing.T) {
	_, err := load.Load(path.Join(samplesDir, "sourceFile.hcl"))
	assert.NoError(t, err)
}