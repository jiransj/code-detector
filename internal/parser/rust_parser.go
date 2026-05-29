package parser

import (
	"regexp"
	"strings"

	"code-detector/internal/model"
)

// RustParser 解析 Rust 源文件
type RustParser struct{}

func NewRustParser() *RustParser { return &RustParser{} }
func (p *RustParser) Language() string { return "rust" }

func (p *RustParser) Globals(filePath string, content []byte) ([]*model.GlobalVariable, error) {
	text := string(content)
	lines := strings.Split(text, "\n")
	commentMask := makeCommentMask(lines, []string{"//"}, [][2]string{{"/*", "*/"}})

	var vars []*model.GlobalVariable
	// static NAME: type = value;
	// const NAME: type = value;
	rustGlobalRegex := regexp.MustCompile(`^\s*(?:(?:pub\s+)?(?:static|const))\s+(?P<name>\w+)\s*:\s*(?P<type>[^=;]+)\s*(?:=|;)`)

	for i, line := range lines {
		if commentMask[i] {
			continue
		}
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.HasPrefix(trimmed, "//") || strings.HasPrefix(trimmed, "/*") || strings.HasPrefix(trimmed, "#") {
			continue
		}
		matches := rustGlobalRegex.FindStringSubmatch(line)
		if matches == nil {
			continue
		}
		nameIdx := rustGlobalRegex.SubexpIndex("name")
		typeIdx := rustGlobalRegex.SubexpIndex("type")
		name := matches[nameIdx]
		varType := ""
		if typeIdx >= 0 && typeIdx < len(matches) {
			varType = strings.TrimSpace(matches[typeIdx])
		}
		if name != "" {
			isConst := strings.Contains(trimmed, " const ")
			vars = append(vars, &model.GlobalVariable{
				Name: name, VarType: varType, Language: "rust",
				FilePath: filePath, LineNum: i + 1, IsConst: isConst,
			})
		}
	}
	return vars, nil
}

var rustFuncRegex = regexp.MustCompile(
	`(?:pub\s+(?:unsafe\s+)?)?(?:unsafe\s+)?fn\s+(?P<name>\w+)\s*\(`,
)
var rustCallRegex = regexp.MustCompile(`(?:(\w+)::)?(\w+)\s*\(`)

func (p *RustParser) Parse(filePath string, content []byte) ([]*model.Function, error) {
	fl := NewFileLinesFromBytes(content)
	lines := fl.Lines()
	text := fl.Text()

	commentMask := makeCommentMask(lines, []string{"//"}, [][2]string{{"/*", "*/"}})
	stringMask := makeStringMask(lines)

	type rsFuncStart struct {
		lineIdx int
		name    string
	}
	var starts []rsFuncStart

	for i, line := range lines {
		if commentMask[i] || stringMask[i] {
			continue
		}
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			continue
		}

		matches := rustFuncRegex.FindStringSubmatch(line)
		if matches == nil {
			continue
		}
		nameIdx := rustFuncRegex.SubexpIndex("name")
		if nameIdx < 0 || nameIdx >= len(matches) {
			continue
		}
		name := matches[nameIdx]
		if name == "" {
			continue
		}

		starts = append(starts, rsFuncStart{lineIdx: i, name: name})
	}

	if len(starts) == 0 {
		return nil, nil
	}

	allFuncs := make([]*model.Function, 0, len(starts))
	for _, fs := range starts {
		// Rust 函数体可能以 { 开始，也可能以 where 子句 + { 开始
		braceOffset := -1
		for j := fs.lineIdx; j < len(lines); j++ {
			line := lines[j]
			idx := strings.Index(line, "{")
			if idx >= 0 && !commentMask[j] {
				braceOffset = fl.LineOffset(j) + idx
				break
			}
		}
		if braceOffset < 0 {
			continue
		}

		closeOffset, err := matchBrace(text, braceOffset)
		if err != nil {
			continue
		}

		startLine := fs.lineIdx
		endLine := fl.LineFromOffset(closeOffset)

		bodyStart := fl.LineOffset(startLine)
		bodyEnd := closeOffset + 1
		if bodyEnd > len(text) {
			bodyEnd = len(text)
		}
		body := text[bodyStart:bodyEnd]

		callStats := extractCallStats(body, rustCallRegex, stringMask, commentMask, startLine, endLine, isRustStdFunction, nil)

		f := &model.Function{
			Name:         fs.name,
			Language:     "rust",
			FilePath:     filePath,
			LineStart:    startLine + 1,
			LineEnd:      endLine + 1,
			Body:         body,
			Dependencies: callStats.Callees,
			CallCount:    callStats.CallCount,
			NestingDepth: callStats.NestingDepth,
		}
		allFuncs = append(allFuncs, f)
	}

	return allFuncs, nil
}

// isRustStdFunction 过滤 Rust 关键字和常用标准库函数
var rustStdKeywords = map[string]bool{
	// Rust 关键字
	"if": true, "else": true, "for": true, "while": true, "loop": true,
	"match": true, "return": true, "break": true, "continue": true,
	"let": true, "mut": true, "const": true, "static": true,
	"fn": true, "impl": true, "trait": true, "struct": true,
	"enum": true, "type": true, "union": true, "mod": true,
	"use": true, "pub": true, "crate": true, "self": true, "super": true,
	"where": true, "as": true, "in": true, "ref": true,
	"move": true, "async": true, "await": true, "unsafe": true,
	"dyn": true, "abstract": true, "become": true, "box": true,
	"do": true, "final": true, "macro": true, "override": true,
	"priv": true, "typeof": true, "unsized": true, "virtual": true,
	"yield": true, "try": true, "macro_rules": true,
	// 基础类型
	"Some": true, "None": true, "Ok": true, "Err": true,
	"String": true, "str": true, "Vec": true, "Box": true,
	"Option": true, "Result": true,
	"HashMap": true, "HashSet": true, "BTreeMap": true, "BTreeSet": true,
	"LinkedList": true, "VecDeque": true, "BinaryHeap": true,
	"Rc": true, "Arc": true, "Cell": true, "RefCell": true, "Mutex": true, "RwLock": true,
	"i32": true, "i64": true, "u32": true, "u64": true, "i128": true, "u128": true,
	"f32": true, "f64": true, "bool": true, "char": true,
	"usize": true, "isize": true, "u8": true, "i8": true, "u16": true, "i16": true,
	// 宏
	"println!": true, "print!": true, "format!": true, "panic!": true,
	"assert!": true, "assert_eq!": true, "assert_ne!": true,
	"unreachable!": true, "unimplemented!": true, "todo!": true,
	"dbg!": true, "vec!": true, "write!": true, "writeln!": true,
	"file!": true, "line!": true, "column!": true,
	"include_str!": true, "include_bytes!": true,
	"stringify!": true, "compile_error!": true, "concat!": true,
	"env!": true, "option_env!": true, "cfg!": true, "cfg_attr!": true,
	"matches!": true, "eprint!": true, "eprintln!": true,
	// Iterator 全部方法
	"map": true, "filter": true, "fold": true, "reduce": true, "for_each": true,
	"collect": true, "partition": true, "find": true, "position": true, "rposition": true,
	"any": true, "all": true, "count": true, "sum": true, "product": true,
	"min": true, "max": true, "min_by": true, "max_by": true, "min_by_key": true, "max_by_key": true,
	"cloned": true, "copied": true, "chain": true, "zip": true, "enumerate": true,
	"skip": true, "take": true, "skip_while": true, "take_while": true,
	"flat_map": true, "flatten": true, "filter_map": true,
	"rev": true, "cycle": true, "peekable": true,
	"inspect": true, "step_by": true,
	"intersperse": true, "intersperse_with": true,
	"is_sorted": true, "is_sorted_by": true, "is_sorted_by_key": true,
	"is_partitioned": true, "try_fold": true, "try_for_each": true,
	"unzip": true, "map_while": true, "map_windows": true,
	// Option/Result 方法
	"expect": true, "unwrap": true, "unwrap_or": true, "unwrap_or_else": true,
	"unwrap_or_default": true, "is_some": true, "is_none": true, "is_ok": true, "is_err": true,
	"ok": true, "err": true, "ok_or": true, "ok_or_else": true,
	"and": true, "and_then": true, "or": true, "or_else": true,
	"map_or": true, "map_or_else": true,
	"as_deref": true, "as_deref_mut": true,
	"transpose": true,
	"get_or_insert": true, "get_or_insert_with": true,
	// String/str 方法
	"len": true, "is_empty": true, "rfind": true,
	"starts_with": true, "ends_with": true,
	"rsplit": true, "split_whitespace": true, "split_ascii_whitespace": true,
	"split_inclusive": true, "rsplit_once": true, "split_once": true,
	"lines": true, "chars": true, "bytes": true, "char_indices": true,
	"as_bytes": true, "as_str": true, "as_mut_str": true,
	"to_string": true, "to_owned": true,
	"to_lowercase": true, "to_uppercase": true,
	"escape_debug": true, "escape_default": true, "escape_unicode": true,
	"trim": true, "trim_start": true, "trim_end": true,
	"trim_matches": true, "trim_start_matches": true, "trim_end_matches": true,
	"matches": true, "rmatches": true, "match_indices": true, "rmatch_indices": true,
	"replacen": true,
	"strip_prefix": true, "strip_suffix": true,
	"parse": true, "clone": true, "default": true,
	// Vec 方法
	"push": true, "pop": true, "insert": true, "remove": true,
	"append": true, "clear": true, "reserve": true, "reserve_exact": true, "resize": true, "resize_with": true,
	"sort": true, "sort_by": true, "sort_by_key": true, "sort_unstable": true, "sort_unstable_by": true, "sort_unstable_by_key": true,
	"reverse": true, "dedup": true, "dedup_by": true, "dedup_by_key": true,
	"splice": true, "drain": true, "retain": true, "retain_mut": true,
	"swap_remove": true, "truncate": true, "extend_from_slice": true,
	"split_off": true, "split_at_spare_mut": true,
	"leak": true, "as_slice": true, "as_mut_slice": true,
	"copy_from_slice": true, "swap_with_slice": true,
	// HashMap 方法
	"entry": true, "or_insert": true, "or_default": true, "and_modify": true,
	"get": true, "get_mut": true, "get_key_value": true,
	"remove_entry": true,
	"iter": true, "iter_mut": true, "into_keys": true, "into_values": true,
	"contains_key": true,
	"shrink_to_fit": true, "shrink_to": true,
	// HashSet 方法
	"difference": true, "symmetric_difference": true, "intersection": true,
	// VecDeque 方法
	"make_contiguous": true,
	"as_slices": true,
	// LinkedList 方法
	"cursor_front": true, "cursor_back": true,
	"drain_filter": true,
	// BinaryHeap 方法
	"peek_mut": true,
	"into_sorted_vec": true, "drain_sorted": true,
	// Rc/Arc 方法
	"downgrade": true, "upgrade": true, "ptr_eq": true,
	"make_mut": true, "unwrap_or_clone": true,
	// Cell/RefCell 方法
	"borrow": true, "borrow_mut": true, "try_borrow": true, "try_borrow_mut": true,
	// Mutex/RwLock 方法
	"lock": true, "try_lock": true, "try_read": true,
	"try_write": true,
	// Path/PathBuf
	"new": true, "as_path": true, "parent": true, "file_name": true, "extension": true, "stem": true,
	"with_extension": true, "with_file_name": true,
	"is_absolute": true, "is_relative": true, "has_root": true,
	"join": true, "components": true, "display": true,
	"canonicalize": true,
	// std::fs
	"read_to_string": true, "read_dir": true,
	"create_dir": true, "create_dir_all": true,
	"remove_file": true, "remove_dir": true, "remove_dir_all": true,
	"rename": true, "copy": true, "hard_link": true,
	"read_link": true, "set_permissions": true,
	// std::io
	"Read": true, "BufRead": true, "Seek": true,
	"BufReader": true, "BufWriter": true, "Lines": true, "Bytes": true,
	"Chain": true, "Take": true, "Cursor": true,
	"stdin": true, "stdout": true, "stderr": true,
	"read_line": true, "read_vectored": true,
	"write_fmt": true, "flush": true,
	// std::net
	"TcpListener": true, "TcpStream": true, "UdpSocket": true,
	"ToSocketAddrs": true, "lookup_host": true,
	"IpAddr": true, "Ipv4Addr": true, "Ipv6Addr": true,
	"SocketAddr": true, "SocketAddrV4": true, "SocketAddrV6": true,
	"Shutdown": true,
	// std::time
	"Duration": true, "Instant": true, "SystemTime": true, "UNIX_EPOCH": true,
	"elapsed": true, "checked_add": true, "checked_sub": true, "saturating_add": true, "saturating_sub": true,
	// std::thread
	"spawn": true, "sleep": true, "yield_now": true, "current": true,
	"park": true, "unpark": true, "available_parallelism": true,
	"Builder": true, "JoinHandle": true, "ThreadId": true,
	// std::sync
	"mpsc": true, "channel": true, "Sender": true, "Receiver": true,
	"sync_channel": true, "SyncSender": true, "TrySendError": true, "TryRecvError": true, "RecvTimeoutError": true,
	"Barrier": true, "BarrierWaitResult": true, "Condvar": true, "Once": true, "OnceLock": true,
	"WaitTimeoutResult": true, "TryLockError": true, "PoisonError": true,
	// std::process
	"Command": true, "Output": true, "Child": true, "ChildStdout": true, "ChildStdin": true, "ChildStderr": true,
	"Stdio": true, "ExitStatus": true, "ExitCode": true,
	"args": true, "var": true, "vars": true, "current_dir": true, "set_current_dir": true, "temp_dir": true,
	"status": true, "output": true,
	// std::env
	"args_os": true, "var_os": true, "vars_os": true,
	"home_dir": true,
	"split_paths": true, "consts": true,
	"set_var": true, "remove_var": true,
	// std::fmt
	"Debug": true, "Display": true, "Write": true, "Formatter": true,
	"format_args": true, "Arguments": true,
	// std::error
	"Error": true, "ErrorKind": true,
	// serde 常用
	"Serialize": true, "Deserialize": true, "serde": true,
	// 常用 crate
	"lazy_static": true, "once_cell": true, "regex": true,
	"tokio": true, "rayon": true, "anyhow": true, "thiserror": true,
}

func isRustKeyword(name string) bool {
	return rustStdKeywords[name]
}

func isRustStdFunction(name string) bool {
	return rustStdKeywords[name]
}
