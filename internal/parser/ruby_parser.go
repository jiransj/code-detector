package parser

import (
	"regexp"
	"strings"

	"code-detector/internal/model"
)

// RubyParser 解析 Ruby 源文件
type RubyParser struct{}

func NewRubyParser() *RubyParser { return &RubyParser{} }
func (p *RubyParser) Language() string { return "ruby" }

func (p *RubyParser) Globals(filePath string, content []byte) ([]*model.GlobalVariable, error) {
	text := string(content)
	lines := strings.Split(text, "\n")
	commentMask := makeCommentMask(lines, []string{"#"}, [][2]string{})

	var vars []*model.GlobalVariable
	rubyGlobalRegex := regexp.MustCompile(`^\s*(?:(?P<name>\$\w+)|(?P<const>[A-Z]\w+))\s*=\s*`)

	for i, line := range lines {
		if commentMask[i] {
			continue
		}
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			continue
		}
		if strings.HasPrefix(trimmed, "def ") || strings.HasPrefix(trimmed, "class ") ||
			strings.HasPrefix(trimmed, "module ") || strings.HasPrefix(trimmed, "if ") ||
			strings.HasPrefix(trimmed, "unless ") || strings.HasPrefix(trimmed, "end") {
			continue
		}
		matches := rubyGlobalRegex.FindStringSubmatch(line)
		if matches == nil {
			continue
		}
		nameIdx := rubyGlobalRegex.SubexpIndex("name")
		constIdx := rubyGlobalRegex.SubexpIndex("const")
		name := matches[nameIdx]
		if name == "" {
			name = matches[constIdx]
		}
		if name != "" {
			isConst := matches[constIdx] != ""
			vars = append(vars, &model.GlobalVariable{
				Name: name, VarType: "unknown", Language: "ruby",
				FilePath: filePath, LineNum: i + 1, IsConst: isConst,
			})
		}
	}
	return vars, nil
}

var rubyFuncRegex = regexp.MustCompile(
	`^\s*(?:def\s+)(?P<name>\w+(?:[?!])?)\s*[\(;]`,
)
var rubyCallRegex = regexp.MustCompile(`(?:(\w+)\.)?(\w+)\s*[\( ]`)

func (p *RubyParser) Parse(filePath string, content []byte) ([]*model.Function, error) {
	text := string(content)
	lines := strings.Split(text, "\n")

	// Ruby 注释 #，没有块注释（=begin/=end 极少用）
	commentMask := makeCommentMask(lines, []string{"#"}, [][2]string{})
	stringMask := makeStringMask(lines)

	type rbFuncStart struct {
		lineIdx int
		name    string
		indent  int
	}
	var starts []rbFuncStart

	for i, line := range lines {
		if commentMask[i] || stringMask[i] {
			continue
		}
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			continue
		}

		matches := rubyFuncRegex.FindStringSubmatch(line)
		if matches == nil {
			continue
		}
		nameIdx := rubyFuncRegex.SubexpIndex("name")
		if nameIdx < 0 || nameIdx >= len(matches) {
			continue
		}
		name := matches[nameIdx]
		if name == "" {
			continue
		}

		indent := countIndent(line)
		starts = append(starts, rbFuncStart{lineIdx: i, name: name, indent: indent})
	}

	if len(starts) == 0 {
		return nil, nil
	}

	allFuncs := make([]*model.Function, 0, len(starts))
	for _, fs := range starts {
		bodyStart := fs.lineIdx + 1
		if bodyStart >= len(lines) {
			continue
		}

		bodyEnd := len(lines) - 1
		foundEnd := false
		for j := bodyStart; j < len(lines); j++ {
			if commentMask[j] || stringMask[j] {
				continue
			}
			trimmed := strings.TrimSpace(lines[j])
			if trimmed == "" {
				continue
			}
			if trimmed == "end" {
				bodyEnd = j
				foundEnd = true
				break
			}
		}
		if !foundEnd {
			continue
		}

		bodyLines := lines[fs.lineIdx : bodyEnd+1]
		body := strings.Join(bodyLines, "\n")

		callStats := extractCallStatsSimple(body, rubyCallRegex, isRubyKeyword)

		f := &model.Function{
			Name:         fs.name,
			Language:     "ruby",
			FilePath:     filePath,
			LineStart:    fs.lineIdx + 1,
			LineEnd:      bodyEnd + 1,
			Body:         body,
			Dependencies: callStats.Callees,
			CallCount:    callStats.CallCount,
			NestingDepth: callStats.NestingDepth,
		}
		allFuncs = append(allFuncs, f)
	}

	return allFuncs, nil
}

func extractRubyCalls(body string, callRegex *regexp.Regexp) []string {
	return extractCallsSimple(body, callRegex, isRubyKeyword)
}

// isRubyKeyword 过滤 Ruby 关键字/内置函数/常用标准库
var rubyKeywords = map[string]bool{
	// Ruby 关键字
	"if": true, "elsif": true, "else": true, "unless": true, "case": true, "when": true,
	"for": true, "while": true, "until": true, "do": true, "end": true, "begin": true,
	"rescue": true, "ensure": true, "raise": true, "throw": true, "catch": true,
	"return": true, "break": true, "next": true, "redo": true, "retry": true,
	"yield": true, "def": true, "class": true, "module": true, "lambda": true, "proc": true,
	"self": true, "super": true, "nil": true, "true": true, "false": true,
	"and": true, "or": true, "not": true, "in": true,
	// Kernel 常用方法
	"puts": true, "print": true, "require": true, "include": true, "extend": true,
	"attr_reader": true, "attr_writer": true, "attr_accessor": true, "alias_method": true,
	"private": true, "protected": true, "public": true, "module_function": true,
	"new": true, "initialize": true, "block_given?": true,
	"loop": true, "tap": true, "then": true,
	"fail": true, "warn": true, "abort": true, "exit": true,
	"sleep": true, "rand": true, "srand": true, "exec": true, "fork": true, "spawn": true,
	"system": true, "open": true, "select": true, "load": true, "eval": true, "trap": true,
	"caller": true, "binding": true, "at_exit": true, "singleton_class": true,
	"respond_to?": true, "is_a?": true, "kind_of?": true, "instance_of?": true,
	"methods": true, "instance_methods": true, "singleton_methods": true,
	"send": true, "public_send": true, "define_method": true,
	"to_s": true, "to_i": true, "to_f": true, "to_a": true, "to_h": true, "to_sym": true,
	"inspect": true, "hash": true, "eql?": true,
	// Enumerable
	"each": true, "map": true, "collect": true, "filter": true,
	"reject": true, "reduce": true, "inject": true, "find": true, "find_all": true,
	"sort": true, "sort_by": true, "all?": true, "any?": true, "none?": true, "one?": true,
	"chunk": true, "chunk_while": true, "collect_concat": true, "count": true, "cycle": true,
	"detect": true, "drop": true, "drop_while": true,
	"each_cons": true, "each_entry": true, "each_slice": true, "each_with_index": true,
	"each_with_object": true, "entries": true, "first": true, "grep": true, "grep_v": true,
	"group_by": true, "include?": true, "lazy": true, "max": true, "max_by": true,
	"member?": true, "min": true, "min_by": true, "minmax": true, "minmax_by": true,
	"partition": true, "reverse_each": true, "slice_after": true, "slice_before": true,
	"slice_when": true, "take": true, "take_while": true, "tally": true,
	"uniq": true, "uniq_by": true, "zip": true, "flat_map": true, "find_index": true,
	"sum": true,
	// Enumerator
	"with_index": true, "with_object": true, "with_proc": true, "peek": true, "feed": true,
	"next_values": true, "peek_values": true, "rewind": true,
	// Array
	"push": true, "pop": true, "shift": true, "unshift": true, "delete": true, "delete_at": true,
	"delete_if": true, "insert": true, "clear": true, "sample": true, "shuffle": true,
	"combination": true, "permutation": true, "repeated_combination": true, "repeated_permutation": true,
	"product": true, "pack": true, "unpack": true, "transpose": true, "values_at": true,
	"flatten": true, "compact": true, "slice": true, "length": true,
	// String
	"gsub": true, "sub": true, "scan": true, "match": true,
	"upcase": true, "downcase": true, "capitalize": true, "swapcase": true,
	"chomp": true, "chop": true, "strip": true, "lstrip": true, "rstrip": true,
	"squeeze": true, "reverse": true,
	"ljust": true, "rjust": true, "center": true, "tr": true, "tr_s": true,
	"each_line": true, "each_byte": true, "each_char": true, "each_codepoint": true,
	"lines": true, "bytes": true, "chars": true, "codepoints": true,
	"empty?": true, "start_with?": true, "end_with?": true,
	"index": true, "rindex": true,
	"split": true, "concat": true, "prepend": true,
	// Hash
	"fetch": true, "store": true, "has_key?": true, "key?": true, "has_value?": true, "value?": true,
	"key": true, "keys": true, "values": true,
	"merge": true, "update": true, "invert": true,
	"transform_keys": true, "transform_values": true, "except": true,
	"dig": true, "assoc": true, "rassoc": true,
	// IO / File
	"read": true, "write": true, "close": true, "gets": true, "readline": true, "readlines": true,
	"seek": true, "pos": true, "tell": true, "stat": true,
	"flock": true, "path": true, "atime": true, "mtime": true, "ctime": true,
	"exists?": true, "file?": true, "directory?": true, "readable?": true, "writable?": true,
	"dirname": true, "basename": true, "extname": true, "join": true, "expand_path": true,
	"absolute_path": true, "relative_path_from": true,
	// Time / Date
	"now": true, "at": true, "local": true, "gm": true, "utc": true, "mktime": true,
	"parse": true, "strftime": true, "iso8601": true, "rfc2822": true, "rfc822": true,
	"year": true, "month": true, "day": true, "hour": true, "sec": true,
	"wday": true, "yday": true, "zone": true, "utc_offset": true,
	"to_date": true, "to_datetime": true, "to_time": true,
	// Math
	"sqrt": true, "sin": true, "cos": true, "tan": true, "asin": true, "acos": true, "atan": true, "atan2": true,
	"sinh": true, "cosh": true, "tanh": true, "asinh": true, "acosh": true, "atanh": true,
	"exp": true, "log": true, "log10": true, "log2": true, "hypot": true,
	"erf": true, "erfc": true, "gamma": true, "lgamma": true, "frexp": true, "ldexp": true, "cbrt": true,
	// Thread
	"start": true, "current": true, "main": true, "pass": true, "kill": true,
	"list": true, "stop": true, "alive?": true, "status": true, "value": true, "wakeup": true,
	"terminate": true, "priority": true, "group": true,
	// Regexp
	"escape": true, "quote": true, "union": true, "last_match": true,
	// JSON
	"JSON": true, "dump": true, "generate": true, "pretty_generate": true,
	// YAML
	"safe_load": true, "safe_load_file": true,
	// ERB
	"ERB": true, "result": true, "run": true, "trim_mode": true,
	// Digest
	"Digest": true, "hexdigest": true, "base64digest": true, "digest": true,
	"MD5": true, "SHA1": true, "SHA256": true, "SHA512": true,
	// Net::HTTP
	"Net": true, "HTTP": true, "get": true, "post": true, "put": true,
	"get_response": true, "post_form": true,
	// ActiveSupport / Rails 常用扩展
	"present?": true, "blank?": true, "presence": true, "try": true, "try!": true, "in?": true,
	"pluralize": true, "singularize": true, "camelize": true, "underscore": true,
	"dasherize": true, "demodulize": true, "tableize": true, "classify": true,
	"constantize": true, "humanize": true, "parameterize": true, "transliterate": true,
	"indent": true, "truncate": true, "truncate_words": true,
	"has_secure_password": true, "has_secure_token": true,
	"belongs_to": true, "has_one": true, "has_many": true, "has_and_belongs_to_many": true,
	"scope": true, "validates": true, "validates_presence_of": true, "validates_uniqueness_of": true,
	"before_action": true, "after_action": true, "skip_before_action": true, "skip_after_action": true,
	"before_validate": true, "after_validate": true, "before_save": true, "after_save": true,
	"before_create": true, "after_create": true, "before_update": true, "after_update": true,
	"before_destroy": true, "after_destroy": true,
	// Rake
	"task": true, "namespace": true, "desc": true, "file": true, "directory": true, "multitask": true,
	"rule": true, "import": true,
}

func isRubyKeyword(name string) bool {
	return rubyKeywords[name]
}

func isRubyStdFunction(name string) bool {
	return rubyKeywords[name]
}
