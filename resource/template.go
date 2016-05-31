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

package resource

// Template is a task defined by content and a destination
type Template struct {
	TemplateName string
	Content      string `hcl:"content"`
	Destination  string `hcl:"destination"`

	renderer *Renderer
	parent   *Module
}

// Name returns the name of this template
func (t *Template) Name() string {
	return t.TemplateName
}

// Validate validates the template config
func (t *Template) Validate() error {
	_, err := t.renderer.Render(t.Content)
	return err
}

// Check satisfies the Monitor interface
func (t *Template) Check() (string, bool, error) {
	return "", false, nil
}

// Apply (plus Check) satisfies the Task interface
func (t *Template) Apply() error {
	return nil
}

// Prepare this module for use
func (t *Template) Prepare(parent *Module) error {
	t.parent = parent

	var err error
	t.renderer, err = NewRenderer(parent)

	return err
}
