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
	"strings"
	"testing"
	"time"
	"unicode"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type translationSample struct {
	str, out string
}

func titleize(s string) string { return strings.ToTitle(s[:1]) + lower(s[1:]) }

func init() {
	AddInitialisms("elb", "cap", "capwd", "wd")
}

func TestIndexOfInitialismsSorted(t *testing.T) {
	configuredInitialisms := map[string]bool{
		"ACL":   true,
		"API":   true,
		"ASCII": true,
		"CPU":   true,
		"CSS":   true,
		"DNS":   true,
		"VM":    true,
		"XML":   true,
		"XMPP":  true,
		"XSRF":  true,
		"XSS":   true,
	}

	goldenSample := []string{
		"ASCII",
		"XMPP",
		"XSRF",
		"ACL",
		"API",
		"CPU",
		"CSS",
		"DNS",
		"XML",
		"XSS",
		"VM",
	}
	for i := 0; i < 50; i++ {
		sample := newIndexOfInitialisms().load(configuredInitialisms).sorted()
		failMsg := "equal sorted initialisms should be always equal"

		if !assert.Equal(t, goldenSample, sample, failMsg) {
			t.FailNow()
		}
	}
}

func TestHighUnicode(t *testing.T) {
	ss := "日本語sample 2 Text"
	rss := []rune(ss)

	require.False(t, rss[0] < unicode.MaxASCII && unicode.IsLetter(rss[0]))
}

const (
	textTitle  = "Text"
	blankText  = " text"
	dashText   = "-text"
	uscoreText = "_text"

	sampleTitle  = "Sample"
	sampleString = "sample"
	sampleBlank  = "sample "
	sampleDash   = "sample-"
	sampleUscore = "sample_"
)

func TestToGoName(t *testing.T) {
	samples := []translationSample{
		{"@Type", "AtType"},
		{"Sample@where", "SampleAtWhere"},
		{"Id", "ID"},
		{"SomethingTTLSeconds", "SomethingTTLSeconds"},
		{"sample text", "SampleText"},
		{"IPv6Address", "IPV6Address"},
		{"IPv4Address", "IPV4Address"},
		{"sample-text", "SampleText"},
		{"sample_text", "SampleText"},
		{"sampleText", "SampleText"},
		{"sample 2 Text", "Sample2Text"},
		{"findThingById", "FindThingByID"},
		{"日本語sample 2 Text", "X日本語sample2Text"},
		{"日本語findThingById", "X日本語findThingByID"},
		{"findTHINGSbyID", "FindTHINGSbyID"},
		{"x-isAnOptionalHeader0", "XIsAnOptionalHeader0"},
		{"get$ref", "GetDollarRef"},
		{"éget$ref", "ÉgetDollarRef"},
		{"日get$ref", "X日getDollarRef"},
		{"", ""},
		{"?", ""},
		{"!", "Bang"},
		{"", ""},
	}

	for _, k := range commonInitialisms.sorted() {
		k = upper(k)

		samples = append(samples,
			translationSample{sampleBlank + lower(k) + blankText, sampleTitle + k + textTitle},
			translationSample{sampleDash + lower(k) + dashText, sampleTitle + k + textTitle},
			translationSample{sampleUscore + lower(k) + uscoreText, sampleTitle + k + textTitle},
			translationSample{sampleString + titleize(k) + textTitle, sampleTitle + k + textTitle},
			translationSample{sampleBlank + lower(k), sampleTitle + k},
			translationSample{sampleDash + lower(k), sampleTitle + k},
			translationSample{sampleUscore + lower(k), sampleTitle + k},
			translationSample{sampleString + titleize(k), sampleTitle + k},
			translationSample{sampleBlank + titleize(k) + blankText, sampleTitle + k + textTitle},
			translationSample{sampleDash + titleize(k) + dashText, sampleTitle + k + textTitle},
			translationSample{sampleUscore + titleize(k) + uscoreText, sampleTitle + k + textTitle},
		)
	}

	for _, sample := range samples {
		result := ToGoName(sample.str)
		assert.Equal(t, sample.out, result,
			"ToGoName(%q) == %q but got %q", sample.str, sample.out, result)
	}
}

func TestContainsStringsCI(t *testing.T) {
	list := []string{"hello", "world", "and", "such"}

	assert.True(t, ContainsStringsCI(list, "hELLo"))
	assert.True(t, ContainsStringsCI(list, "world"))
	assert.True(t, ContainsStringsCI(list, "AND"))
	assert.False(t, ContainsStringsCI(list, "nuts"))
}

func TestContainsStrings(t *testing.T) {
	list := []string{"hello", "world", "and", "such"}

	assert.True(t, ContainsStrings(list, "hello"))
	assert.False(t, ContainsStrings(list, "hELLo"))
	assert.True(t, ContainsStrings(list, "world"))
	assert.False(t, ContainsStrings(list, "World"))
	assert.True(t, ContainsStrings(list, "and"))
	assert.False(t, ContainsStrings(list, "AND"))
	assert.False(t, ContainsStrings(list, "nuts"))
}

const (
	collectionFormatComma = "csv"
)

func TestSplitByFormat(t *testing.T) {
	expected := []string{"one", "two", "three"}
	for _, fmt := range []string{collectionFormatComma, collectionFormatPipe, collectionFormatTab, collectionFormatSpace, collectionFormatMulti} {

		var actual []string
		switch fmt {
		case collectionFormatMulti:
			assert.Nil(t, SplitByFormat("", fmt))
			assert.Nil(t, SplitByFormat("blah", fmt))
		case collectionFormatSpace:
			actual = SplitByFormat(strings.Join(expected, " "), fmt)
			assert.EqualValues(t, expected, actual)
		case collectionFormatPipe:
			actual = SplitByFormat(strings.Join(expected, "|"), fmt)
			assert.EqualValues(t, expected, actual)
		case collectionFormatTab:
			actual = SplitByFormat(strings.Join(expected, "\t"), fmt)
			assert.EqualValues(t, expected, actual)
		default:
			actual = SplitByFormat(strings.Join(expected, ","), fmt)
			assert.EqualValues(t, expected, actual)
		}
	}
}

func TestJoinByFormat(t *testing.T) {
	for _, fmt := range []string{collectionFormatComma, collectionFormatPipe, collectionFormatTab, collectionFormatSpace, collectionFormatMulti} {

		lval := []string{"one", "two", "three"}
		var expected []string
		switch fmt {
		case collectionFormatMulti:
			expected = lval
		case collectionFormatSpace:
			expected = []string{strings.Join(lval, " ")}
		case collectionFormatPipe:
			expected = []string{strings.Join(lval, "|")}
		case collectionFormatTab:
			expected = []string{strings.Join(lval, "\t")}
		default:
			expected = []string{strings.Join(lval, ",")}
		}
		assert.Nil(t, JoinByFormat(nil, fmt))
		assert.EqualValues(t, expected, JoinByFormat(lval, fmt))
	}
}

func TestToFileName(t *testing.T) {
	samples := []translationSample{
		{"SampleText", "sample_text"},
		{"FindThingByID", "find_thing_by_id"},
		{"CAPWD.folwdBylc", "capwd_folwd_bylc"},
		{"CAPWDfolwdBylc", "cap_w_dfolwd_bylc"},
		{"CAP_WD_folwdBylc", "cap_wd_folwd_bylc"},
		{"TypeOAI_alias", "type_oai_alias"},
		{"Type_OAI_alias", "type_oai_alias"},
		{"Type_OAIAlias", "type_oai_alias"},
		{"ELB.HTTPLoadBalancer", "elb_http_load_balancer"},
		{"elbHTTPLoadBalancer", "elb_http_load_balancer"},
		{"ELBHTTPLoadBalancer", "elb_http_load_balancer"},
		{"get$Ref", "get_dollar_ref"},
	}
	for _, k := range commonInitialisms.sorted() {
		samples = append(samples,
			translationSample{sampleTitle + k + textTitle, sampleUscore + lower(k) + uscoreText},
		)
	}

	for _, sample := range samples {
		result := ToFileName(sample.str)
		assert.Equal(t, sample.out, ToFileName(sample.str),
			"ToFileName(%q) == %q but got %q", sample.str, sample.out, result)
	}
}

func TestToCommandName(t *testing.T) {
	samples := []translationSample{
		{"SampleText", "sample-text"},
		{"FindThingByID", "find-thing-by-id"},
		{"elbHTTPLoadBalancer", "elb-http-load-balancer"},
		{"get$ref", "get-dollar-ref"},
		{"get!ref", "get-bang-ref"},
	}

	for _, k := range commonInitialisms.sorted() {
		samples = append(samples,
			translationSample{sampleTitle + k + textTitle, sampleDash + lower(k) + dashText},
		)
	}

	for _, sample := range samples {
		assert.Equal(t, sample.out, ToCommandName(sample.str))
	}
}

func TestToHumanName(t *testing.T) {
	samples := []translationSample{
		{"Id", "Id"},
		{"SampleText", "sample text"},
		{"FindThingByID", "find thing by ID"},
		{"elbHTTPLoadBalancer", "elb HTTP load balancer"},
	}

	for _, k := range commonInitialisms.sorted() {
		samples = append(samples,
			translationSample{sampleTitle + k + textTitle, sampleBlank + k + blankText},
		)
	}

	for _, sample := range samples {
		assert.Equal(t, sample.out, ToHumanNameLower(sample.str))
	}
}

func TestToJSONName(t *testing.T) {
	samples := []translationSample{
		{"SampleText", "sampleText"},
		{"FindThingByID", "findThingById"},
		{"elbHTTPLoadBalancer", "elbHttpLoadBalancer"},
		{"get$ref", "getDollarRef"},
		{"get!ref", "getBangRef"},
	}

	for _, k := range commonInitialisms.sorted() {
		samples = append(samples,
			translationSample{sampleTitle + k + textTitle, sampleString + titleize(k) + textTitle},
		)
	}

	for _, sample := range samples {
		assert.Equal(t, sample.out, ToJSONName(sample.str))
	}
}

type SimpleZeroes struct {
	ID   string
	Name string
}
type ZeroesWithTime struct {
	Time time.Time
}

type dummyZeroable struct {
	zero bool
}

func (d dummyZeroable) IsZero() bool {
	return d.zero
}

func TestIsZero(t *testing.T) {
	var strs [5]string
	var strss []string
	var a int
	var b int8
	var c int16
	var d int32
	var e int64
	var f uint
	var g uint8
	var h uint16
	var i uint32
	var j uint64
	var k map[string]string
	var l interface{}
	var m *SimpleZeroes
	var n string
	var o SimpleZeroes
	var p ZeroesWithTime
	var q time.Time
	data := []struct {
		Data     interface{}
		Expected bool
	}{
		{a, true},
		{b, true},
		{c, true},
		{d, true},
		{e, true},
		{f, true},
		{g, true},
		{h, true},
		{i, true},
		{j, true},
		{k, true},
		{l, true},
		{m, true},
		{n, true},
		{o, true},
		{p, true},
		{q, true},
		{strss, true},
		{strs, true},
		{"", true},
		{nil, true},
		{1, false},
		{0, true},
		{int8(1), false},
		{int8(0), true},
		{int16(1), false},
		{int16(0), true},
		{int32(1), false},
		{int32(0), true},
		{int64(1), false},
		{int64(0), true},
		{uint(1), false},
		{uint(0), true},
		{uint8(1), false},
		{uint8(0), true},
		{uint16(1), false},
		{uint16(0), true},
		{uint32(1), false},
		{uint32(0), true},
		{uint64(1), false},
		{uint64(0), true},
		{0.0, true},
		{0.1, false},
		{float32(0.0), true},
		{float32(0.1), false},
		{float64(0.0), true},
		{float64(0.1), false},
		{[...]string{}, true},
		{[...]string{"hello"}, false},
		{[]string(nil), true},
		{[]string{"a"}, false},
		{&dummyZeroable{true}, true},
		{&dummyZeroable{false}, false},
		{(*dummyZeroable)(nil), true},
	}

	for _, it := range data {
		assert.Equalf(t, it.Expected, IsZero(it.Data), "expected %#v, but got %#v", it.Expected, it.Data)
	}
}

func TestCamelize(t *testing.T) {
	samples := []translationSample{
		{"SampleText", "Sampletext"},
		{"FindThingByID", "Findthingbyid"},
		{"CAPWD.folwdBylc", "Capwd.folwdbylc"},
		{"CAPWDfolwdBylc", "Capwdfolwdbylc"},
		{"CAP_WD_folwdBylc", "Cap_wd_folwdbylc"},
		{"TypeOAI_alias", "Typeoai_alias"},
		{"Type_OAI_alias", "Type_oai_alias"},
		{"Type_OAIAlias", "Type_oaialias"},
		{"ELB.HTTPLoadBalancer", "Elb.httploadbalancer"},
		{"elbHTTPLoadBalancer", "Elbhttploadbalancer"},
		{"ELBHTTPLoadBalancer", "Elbhttploadbalancer"},
		{"12ab", "12ab"},
		{"get$Ref", "Get$ref"},
		{"get!Ref", "Get!ref"},
	}

	for _, sample := range samples {
		res := Camelize(sample.str)
		assert.Equalf(t, sample.out, res, "expected Camelize(%q)=%q, got %q", sample.str, sample.out, res)
	}
}

func TestToHumanNameTitle(t *testing.T) {
	samples := []translationSample{
		{"SampleText", "Sample Text"},
		{"FindThingByID", "Find Thing By ID"},
		{"CAPWD.folwdBylc", "CAPWD Folwd Bylc"},
		{"CAPWDfolwdBylc", "CAP W Dfolwd Bylc"},
		{"CAP_WD_folwdBylc", "CAP WD Folwd Bylc"},
		{"TypeOAI_alias", "Type OAI Alias"},
		{"Type_OAI_alias", "Type OAI Alias"},
		{"Type_OAIAlias", "Type OAI Alias"},
		{"ELB.HTTPLoadBalancer", "ELB HTTP Load Balancer"},
		{"elbHTTPLoadBalancer", "elb HTTP Load Balancer"},
		{"ELBHTTPLoadBalancer", "ELB HTTP Load Balancer"},
		{"get$ref", "Get Dollar Ref"},
		{"get!ref", "Get Bang Ref"},
	}

	for _, sample := range samples {
		res := ToHumanNameTitle(sample.str)
		assert.Equalf(t, sample.out, res, "expected ToHumanNameTitle(%q)=%q, got %q", sample.str, sample.out, res)
	}
}

func TestToVarName(t *testing.T) {
	samples := []translationSample{
		{"SampleText", "sampleText"},
		{"FindThingByID", "findThingByID"},
		{"CAPWD.folwdBylc", "cAPWDFolwdBylc"},
		{"CAPWDfolwdBylc", "cAPWDfolwdBylc"},
		{"CAP_WD_folwdBylc", "cAPWDFolwdBylc"},
		{"TypeOAI_alias", "typeOAIAlias"},
		{"Type_OAI_alias", "typeOAIAlias"},
		{"Type_OAIAlias", "typeOAIAlias"},
		{"ELB.HTTPLoadBalancer", "eLBHTTPLoadBalancer"},
		{"elbHTTPLoadBalancer", "eLBHTTPLoadBalancer"},
		{"ELBHTTPLoadBalancer", "eLBHTTPLoadBalancer"},
		{"Id", "id"},
		{"HTTP", "http"},
		{"A", "a"},
		{"get$ref", "getDollarRef"},
		{"get!ref", "getBangRef"},
	}

	for _, sample := range samples {
		res := ToVarName(sample.str)
		assert.Equalf(t, sample.out, res, "expected ToVarName(%q)=%q, got %q", sample.str, sample.out, res)
	}
}

func TestToGoNameUnicode(t *testing.T) {
	defer func() { GoNamePrefixFunc = nil }()
	GoNamePrefixFunc = func(name string) string {
		// this is the pascalize func from go-swagger codegen
		arg := []rune(name)
		if len(arg) == 0 || arg[0] > '9' {
			return ""
		}
		if arg[0] == '+' {
			return "Plus"
		}
		if arg[0] == '-' {
			return "Minus"
		}

		return "Nr"
	}

	samples := []translationSample{
		{"123_a", "Nr123a"},
		{"!123_a", "Bang123a"},
		{"+123_a", "Plus123a"},
		{"abc", "Abc"},
		{"éabc", "Éabc"},
		{":éabc", "Éabc"},
		{"get$ref", "GetDollarRef"},
		{"get!ref", "GetBangRef"},
		{"get&ref", "GetAndRef"},
		{"get|ref", "GetPipeRef"},
		// TODO: non unicode char
	}

	for _, sample := range samples {
		assert.Equal(t, sample.out, ToGoName(sample.str))
	}
}
