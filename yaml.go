// Copyright 2015 go-swagger maintainers
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//    http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package swag

import (
	"encoding/json"
	"fmt"
	"path/filepath"
	"reflect"
	"sort"
	"strconv"

	"github.com/mailru/easyjson/jlexer"
	"github.com/mailru/easyjson/jwriter"
	yaml "gopkg.in/yaml.v3"
)

// YAMLMatcher matches yaml
func YAMLMatcher(path string) bool {
	ext := filepath.Ext(path)
	return ext == ".yaml" || ext == ".yml"
}

// YAMLToJSON converts YAML unmarshaled data into json compatible data
func YAMLToJSON(data interface{}) (json.RawMessage, error) {
	jm, err := transformData(data)
	if err != nil {
		return nil, err
	}
	b, err := WriteJSON(jm)
	return json.RawMessage(b), err
}

// BytesToYAMLDoc converts a byte slice into a YAML document
func BytesToYAMLDoc(data []byte) (interface{}, error) {
	var document yaml.Node // preserve order that is present in the document
	if err := yaml.Unmarshal(data, &document); err != nil {
		return nil, err
	}
	if document.Kind != yaml.DocumentNode || len(document.Content) != 1 || document.Content[0].Kind != yaml.MappingNode {
		return nil, fmt.Errorf("only YAML documents that are objects are supported: %w", ErrYAML)
	}
	return &document, nil
}

func yamlNode(root *yaml.Node, hooks ...yamlHook) (interface{}, error) {
	switch root.Kind {
	case yaml.DocumentNode:
		return yamlDocument(root, hooks...)
	case yaml.SequenceNode:
		return yamlSequence(root, hooks...)
	case yaml.MappingNode:
		return yamlMapping(root, hooks...)
	case yaml.ScalarNode:
		return yamlScalar(root, hooks...)
	case yaml.AliasNode:
		return yamlNode(root.Alias, hooks...)
	default:
		return nil, fmt.Errorf("unsupported YAML node type: %v: %w", root.Kind, ErrYAML)
	}
}

func yamlDocument(node *yaml.Node, hooks ...yamlHook) (interface{}, error) {
	if len(node.Content) != 1 {
		return nil, fmt.Errorf("unexpected YAML Document node content length: %d: %w", len(node.Content), ErrYAML)
	}
	// TODO: hooks
	return yamlNode(node.Content[0])
}

func yamlMapping(node *yaml.Node, hooks ...yamlHook) (interface{}, error) {
	const sensibleAllocDivider = 2
	m := make(JSONMapSlice, len(node.Content)/sensibleAllocDivider)

	var j int
	for i := 0; i < len(node.Content); i += 2 {
		var nmi JSONMapItem
		k, err := yamlStringScalarC(node.Content[i])
		if err != nil {
			return nil, fmt.Errorf("unable to decode YAML map key: %w: %w", err, ErrYAML)
		}
		nmi.Key = k
		v, err := yamlNode(node.Content[i+1], hooks...)
		if err != nil {
			return nil, fmt.Errorf("unable to process YAML map value for key %q: %w: %w", k, err, ErrYAML)
		}
		nmi.Value = v
		m[j] = nmi
		j++
	}
	return m, nil
}

func yamlSequence(node *yaml.Node, hooks ...yamlHook) (interface{}, error) {
	s := make([]interface{}, 0)

	for i := 0; i < len(node.Content); i++ {

		v, err := yamlNode(node.Content[i], hooks...)
		if err != nil {
			return nil, fmt.Errorf("unable to decode YAML sequence value: %w: %w", err, ErrYAML)
		}
		s = append(s, v)
	}
	return s, nil
}

const ( // See https://yaml.org/type/
	yamlStringScalar = "tag:yaml.org,2002:str"
	yamlIntScalar    = "tag:yaml.org,2002:int"
	yamlBoolScalar   = "tag:yaml.org,2002:bool"
	yamlFloatScalar  = "tag:yaml.org,2002:float"
	yamlTimestamp    = "tag:yaml.org,2002:timestamp"
	yamlNull         = "tag:yaml.org,2002:null"
)

func yamlScalar(node *yaml.Node, hooks ...yamlHook) (interface{}, error) {
	switch node.LongTag() {
	case yamlStringScalar:
		return node.Value, nil
	case yamlBoolScalar:
		b, err := strconv.ParseBool(node.Value)
		if err != nil {
			return nil, fmt.Errorf("unable to process scalar node. Got %q. Expecting bool content: %w: %w", node.Value, err, ErrYAML)
		}
		return b, nil
	case yamlIntScalar:
		i, err := strconv.ParseInt(node.Value, 10, 64)
		if err != nil {
			return nil, fmt.Errorf("unable to process scalar node. Got %q. Expecting integer content: %w: %w", node.Value, err, ErrYAML)
		}
		return i, nil
	case yamlFloatScalar:
		f, err := strconv.ParseFloat(node.Value, 64)
		if err != nil {
			return nil, fmt.Errorf("unable to process scalar node. Got %q. Expecting float content: %w: %w", node.Value, err, ErrYAML)
		}
		return f, nil
	case yamlTimestamp:
		return node.Value, nil
	case yamlNull:
		return nil, nil //nolint:nilnil
	default:
		return nil, fmt.Errorf("YAML tag %q is not supported: %w", node.LongTag(), ErrYAML)
	}
}

func yamlStringScalarC(node *yaml.Node) (string, error) {
	if node.Kind != yaml.ScalarNode {
		return "", fmt.Errorf("expecting a string scalar but got %q: %w", node.Kind, ErrYAML)
	}
	switch node.LongTag() {
	case yamlStringScalar, yamlIntScalar, yamlFloatScalar:
		return node.Value, nil
	default:
		return "", fmt.Errorf("YAML tag %q is not supported as map key: %w", node.LongTag(), ErrYAML)
	}
}

// JSONMapSlice represent a JSON object, with the order of keys maintained
type JSONMapSlice []JSONMapItem

// MarshalJSON renders a JSONMapSlice as JSON
func (s JSONMapSlice) MarshalJSON() ([]byte, error) {
	w := &jwriter.Writer{Flags: jwriter.NilMapAsEmpty | jwriter.NilSliceAsEmpty}
	s.MarshalEasyJSON(w)
	return w.BuildBytes()
}

// MarshalEasyJSON renders a JSONMapSlice as JSON, using easyJSON
func (s JSONMapSlice) MarshalEasyJSON(w *jwriter.Writer) {
	w.RawByte('{')

	ln := len(s)
	last := ln - 1
	for i := 0; i < ln; i++ {
		s[i].MarshalEasyJSON(w)
		if i != last { // last item
			w.RawByte(',')
		}
	}

	w.RawByte('}')
}

// UnmarshalJSON makes a JSONMapSlice from JSON
func (s *JSONMapSlice) UnmarshalJSON(data []byte) error {
	return s.unmarshalJSONWithHooks(data)
}

func (s *JSONMapSlice) unmarshalJSONWithHooks(data []byte, hooks ...jsonHook) error {
	l := jlexer.Lexer{Data: data}
	s.unmarshalEasyJSONWithHooks(&l, hooks...)
	return l.Error()
}

// UnmarshalEasyJSON makes a JSONMapSlice from JSON, using easyJSON
func (s *JSONMapSlice) UnmarshalEasyJSON(in *jlexer.Lexer) {
	s.unmarshalEasyJSONWithHooks(in)
}

func (s *JSONMapSlice) unmarshalEasyJSONWithHooks(in *jlexer.Lexer, hooks ...jsonHook) {
	if in.IsNull() {
		in.Skip()
		return
	}

	var result JSONMapSlice
	in.Delim('{')
	for !in.IsDelim('}') {
		var mi JSONMapItem
		mi.unmarshalEasyJSONWithHooks(in, hooks...)
		result = append(result, mi)
	}
	*s = result
}

func (s JSONMapSlice) MarshalYAML() (interface{}, error) {
	var n yaml.Node
	n.Kind = yaml.DocumentNode
	var nodes []*yaml.Node
	for _, item := range s {
		nn, err := json2yaml(item.Value)
		if err != nil {
			return nil, err
		}
		ns := []*yaml.Node{
			{
				Kind:  yaml.ScalarNode,
				Tag:   yamlStringScalar,
				Value: item.Key,
			},
			nn,
		}
		nodes = append(nodes, ns...)
	}

	n.Content = []*yaml.Node{
		{
			Kind:    yaml.MappingNode,
			Content: nodes,
		},
	}

	return yaml.Marshal(&n)
}

func isNil(input interface{}) bool {
	if input == nil {
		return true
	}
	kind := reflect.TypeOf(input).Kind()
	switch kind { //nolint:exhaustive
	case reflect.Ptr, reflect.Map, reflect.Slice, reflect.Chan:
		return reflect.ValueOf(input).IsNil()
	default:
		return false
	}
}

func json2yaml(item interface{}) (*yaml.Node, error) {
	if isNil(item) {
		return &yaml.Node{
			Kind:  yaml.ScalarNode,
			Value: "null",
		}, nil
	}

	switch val := item.(type) {
	case JSONMapSlice:
		var n yaml.Node
		n.Kind = yaml.MappingNode
		for i := range val {
			childNode, err := json2yaml(&val[i].Value)
			if err != nil {
				return nil, err
			}
			n.Content = append(n.Content, &yaml.Node{
				Kind:  yaml.ScalarNode,
				Tag:   yamlStringScalar,
				Value: val[i].Key,
			}, childNode)
		}
		return &n, nil
	case map[string]interface{}:
		var n yaml.Node
		n.Kind = yaml.MappingNode
		keys := make([]string, 0, len(val))
		for k := range val {
			keys = append(keys, k)
		}
		sort.Strings(keys)

		for _, k := range keys {
			v := val[k]
			childNode, err := json2yaml(v)
			if err != nil {
				return nil, err
			}
			n.Content = append(n.Content, &yaml.Node{
				Kind:  yaml.ScalarNode,
				Tag:   yamlStringScalar,
				Value: k,
			}, childNode)
		}
		return &n, nil
	case []interface{}:
		var n yaml.Node
		n.Kind = yaml.SequenceNode
		for i := range val {
			childNode, err := json2yaml(val[i])
			if err != nil {
				return nil, err
			}
			n.Content = append(n.Content, childNode)
		}
		return &n, nil
	case string:
		return &yaml.Node{
			Kind:  yaml.ScalarNode,
			Tag:   yamlStringScalar,
			Value: val,
		}, nil
	case float64:
		return &yaml.Node{
			Kind:  yaml.ScalarNode,
			Tag:   yamlFloatScalar,
			Value: strconv.FormatFloat(val, 'f', -1, 64),
		}, nil
	case int64:
		return &yaml.Node{
			Kind:  yaml.ScalarNode,
			Tag:   yamlIntScalar,
			Value: strconv.FormatInt(val, 10),
		}, nil
	case uint64:
		return &yaml.Node{
			Kind:  yaml.ScalarNode,
			Tag:   yamlIntScalar,
			Value: strconv.FormatUint(val, 10),
		}, nil
	case bool:
		return &yaml.Node{
			Kind:  yaml.ScalarNode,
			Tag:   yamlBoolScalar,
			Value: strconv.FormatBool(val),
		}, nil
	default:
		return nil, fmt.Errorf("unhandled type: %T: %w", val, ErrYAML)
	}
}

// JSONMapItem represents the value of a key in a JSON object held by JSONMapSlice
type JSONMapItem struct {
	Key   string
	Value interface{}
}

type JSONSliceItem struct {
	Value interface{} // TODO: marshal /unmarshal
}

// MarshalJSON renders a JSONMapItem as JSON
func (s JSONMapItem) MarshalJSON() ([]byte, error) {
	w := &jwriter.Writer{Flags: jwriter.NilMapAsEmpty | jwriter.NilSliceAsEmpty}
	s.MarshalEasyJSON(w)
	return w.BuildBytes()
}

// MarshalEasyJSON renders a JSONMapItem as JSON, using easyJSON
func (s JSONMapItem) MarshalEasyJSON(w *jwriter.Writer) {
	w.String(s.Key)
	w.RawByte(':')
	w.Raw(WriteJSON(s.Value))
}

// UnmarshalJSON makes a JSONMapItem from JSON
func (s *JSONMapItem) UnmarshalJSON(data []byte) error {
	return s.unmarshalJSONWithHooks(data)
}

func (s *JSONMapItem) unmarshalJSONWithHooks(data []byte) error {
	l := jlexer.Lexer{Data: data}
	s.UnmarshalEasyJSON(&l)
	return l.Error()
}

// UnmarshalEasyJSON makes a JSONMapItem from JSON, using easyJSON
func (s *JSONMapItem) UnmarshalEasyJSON(in *jlexer.Lexer) {
	s.unmarshalEasyJSONWithHooks(in)
}

func (s *JSONMapItem) unmarshalEasyJSONWithHooks(in *jlexer.Lexer, hooks ...jsonHook) {
	// TODO: hooks
	key := in.UnsafeString()
	in.WantColon()

	var value interface{}
	switch {
	case in.IsDelim('{'):
		// contains an object
		var inner JSONMapSlice
		inner.unmarshalEasyJSONWithHooks(in, hooks...)
		value = inner

	case in.IsDelim('['):
		// contains an array
		var element JSONSliceItem
		element.unmarshalEasyJSONWithHooks(in, hooks)
		value = element

	default:
		// contains a scalar
		value = in.Interface()
	}

	in.WantComma()
	if !in.Ok() {
		return
	}
	s.Key = key
	s.Value = value
}

func transformData(input interface{}, hooks ...yamlHook) (out interface{}, err error) {
	format := func(t interface{}) (string, error) {
		switch k := t.(type) {
		case string:
			return k, nil
		case uint:
			return strconv.FormatUint(uint64(k), 10), nil
		case uint8:
			return strconv.FormatUint(uint64(k), 10), nil
		case uint16:
			return strconv.FormatUint(uint64(k), 10), nil
		case uint32:
			return strconv.FormatUint(uint64(k), 10), nil
		case uint64:
			return strconv.FormatUint(k, 10), nil
		case int:
			return strconv.Itoa(k), nil
		case int8:
			return strconv.FormatInt(int64(k), 10), nil
		case int16:
			return strconv.FormatInt(int64(k), 10), nil
		case int32:
			return strconv.FormatInt(int64(k), 10), nil
		case int64:
			return strconv.FormatInt(k, 10), nil
		default:
			return "", fmt.Errorf("unexpected map key type, got: %T: %w", k, ErrYAML)
		}
	}

	switch in := input.(type) {
	case yaml.Node:
		return yamlNode(&in, hooks...)
	case *yaml.Node:
		return yamlNode(in, hooks...)
	case map[interface{}]interface{}:
		o := make(JSONMapSlice, 0, len(in))
		for ke, va := range in {
			var nmi JSONMapItem
			if nmi.Key, err = format(ke); err != nil {
				return nil, err
			}

			v, ert := transformData(va, hooks...)
			if ert != nil {
				return nil, ert
			}
			nmi.Value = v
			o = append(o, nmi)
		}
		return o, nil
	case []interface{}:
		len1 := len(in)
		o := make([]interface{}, len1)
		for i := 0; i < len1; i++ {
			o[i], err = transformData(in[i], hooks...)
			if err != nil {
				return nil, err
			}
		}
		return o, nil
	}
	return input, nil
}

type (
	DocProcessor func(any) (any, error)
	DocOption    func(*docOptions)
	docOptions   struct {
		processors []DocProcessor
	}

	yamlHook func()
	jsonHook func()
)

func WithDocProcessor(processor func(any) (any, error)) DocOption {
	return func(o *docOptions) {
		o.processors = append(o.processors, processor)
	}
}

func WithXOrderProcessor(enabled bool) DocOption {
	if !enabled {
		return func(o *docOptions) {}
	}

	return WithDocProcessor(func(doc any) (any, error) {
		switch doc.(type) {
		case yaml.Node:
			// TODO
			return nil, nil // TODO
		case *yaml.Node:
			return nil, nil // TODO
		case JSONMapSlice:
			return nil, nil // TODO
		default:
			return nil, fmt.Errorf(
				"XOrder processor only support yamlv3.Node and swag.JSONMapSlice input documents, got: %T",
				doc,
			)
		}
	})
}

func docOptionsWithDefaults(opts []DocOption) docOptions {
	o := docOptions{}

	for _, apply := range opts {
		apply(&o)
	}

	return o
}

func (o docOptions) HasTransforms() bool {
	return len(o.processors) > 0
}

func (o docOptions) ApplyTransforms(inputDoc any) (doc any, err error) {
	doc = inputDoc
	for _, process := range o.processors {
		doc, err = process(doc)
		if err != nil {
			return nil, err
		}
	}

	return doc, nil
}

// YAMLDoc loads a yaml document from either http or a file and converts it to json
func YAMLDoc(path string, opts ...DocOption) (json.RawMessage, error) {
	yamlDoc, err := YAMLData(path)
	if err != nil {
		return nil, err
	}

	o := docOptionsWithDefaults(opts)
	doc, err := o.ApplyTransforms(yamlDoc)
	if err != nil {
		return nil, err
	}

	data, err := YAMLToJSON(doc)
	if err != nil {
		return nil, err
	}

	return data, nil
}

// YAMLData loads a yaml document from either http or a file
func YAMLData(path string) (interface{}, error) {
	data, err := LoadFromFileOrHTTP(path)
	if err != nil {
		return nil, err
	}

	return BytesToYAMLDoc(data)
}
