package parser

import (
	"regexp"
	"strings"

	"code-detector/internal/model"
)

// PythonParser 解析 Python 源文件
type PythonParser struct{}

func NewPythonParser() *PythonParser { return &PythonParser{} }

func (p *PythonParser) Language() string { return "python" }

func (p *PythonParser) Globals(filePath string, content []byte) ([]*model.GlobalVariable, error) {
	text := string(content)
	lines := strings.Split(text, "\n")
	commentMask := makePythonCommentMask(lines)

	var vars []*model.GlobalVariable
	pyVarRegex := regexp.MustCompile(`^\s*(?P<name>\w+)\s*(?::\s*(?P<type>\w+(?:\[.*?\])?))?\s*=\s*`)

	for i, line := range lines {
		if commentMask[i] {
			continue
		}
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			continue
		}
		if strings.HasPrefix(line, " ") || strings.HasPrefix(line, "\t") {
			continue
		}
		if strings.HasPrefix(trimmed, "def ") || strings.HasPrefix(trimmed, "class ") ||
			strings.HasPrefix(trimmed, "@") || strings.HasPrefix(trimmed, "import ") ||
			strings.HasPrefix(trimmed, "from ") || strings.HasPrefix(trimmed, "return ") ||
			strings.HasPrefix(trimmed, "if ") || strings.HasPrefix(trimmed, "elif ") ||
			strings.HasPrefix(trimmed, "else") || strings.HasPrefix(trimmed, "for ") ||
			strings.HasPrefix(trimmed, "while ") || strings.HasPrefix(trimmed, "try") ||
			strings.HasPrefix(trimmed, "except") || strings.HasPrefix(trimmed, "with ") {
			continue
		}

		matches := pyVarRegex.FindStringSubmatch(line)
		if matches == nil {
			continue
		}
		nameIdx := pyVarRegex.SubexpIndex("name")
		typeIdx := pyVarRegex.SubexpIndex("type")
		name := matches[nameIdx]
		varType := ""
		if typeIdx >= 0 && typeIdx < len(matches) {
			varType = matches[typeIdx]
		}
		if name != "" && !isPyKeyword(name) {
			vars = append(vars, &model.GlobalVariable{
				Name:     name,
				VarType:  varType,
				Language: "python",
				FilePath: filePath,
				LineNum:  i + 1,
				IsConst:  strings.ToUpper(name) == name && len(name) > 1,
			})
		}
	}
	return vars, nil
}

// pyFuncRegex 匹配 def 定义行（必须在行首非空字符开始）
var pyFuncRegex = regexp.MustCompile(`^\s*def\s+(?P<name>\w+)\s*\(`)

// pyCallRegex 匹配函数调用
var pyCallRegex = regexp.MustCompile(`(?:(\w+)\.)?(\w+)\s*\(`)

func (p *PythonParser) Parse(filePath string, content []byte) ([]*model.Function, error) {
	text := string(content)
	lines := strings.Split(text, "\n")

	// Python 注释 + 多行字符串
	commentMask := makePythonCommentMask(lines)
	stringMask := makePythonStringMask(lines)

	// 定位函数定义
	type pyFuncStart struct {
		lineIdx  int
		name     string
		indent   int // 函数定义行的缩进级别
	}
	var starts []pyFuncStart

	for i, line := range lines {
		if commentMask[i] || stringMask[i] {
			continue
		}
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			continue
		}
		matches := pyFuncRegex.FindStringSubmatch(line)
		if matches == nil {
			continue
		}
		nameIdx := pyFuncRegex.SubexpIndex("name")
		if nameIdx < 0 || nameIdx >= len(matches) {
			continue
		}
		name := matches[nameIdx]
		if name == "" {
			continue
		}

		indent := countIndent(line)
		starts = append(starts, pyFuncStart{
			lineIdx: i,
			name:    name,
			indent:  indent,
		})
	}

	if len(starts) == 0 {
		return nil, nil
	}

	allFuncs := make([]*model.Function, 0, len(starts))

	for _, fs := range starts {
		// Python 函数体结束条件：遇到缩进 <= 本函数定义缩进且非空/非注释/非装饰器的行
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
			// 空行或装饰器不结束
			if strings.HasPrefix(trimmed, "@") {
				continue
			}
			indent := countIndent(lines[j])
			if indent <= fs.indent {
				bodyEnd = j - 1
				foundEnd = true
				break
			}
		}
		if !foundEnd {
			bodyEnd = len(lines) - 1
		}

		// 提取函数体
		bodyLines := lines[fs.lineIdx : bodyEnd+1]
		body := strings.Join(bodyLines, "\n")

		// 提取调用统计（只在函数体内）
		callStats := extractCallStatsSimple(body, pyCallRegex, func(name string) bool {
			return isPyKeyword(name) || name[0] == '_'
		})

		f := &model.Function{
			Name:         fs.name,
			Language:     "python",
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

// makePythonCommentMask 标记注释行（# 及多行字符串中的行）
func makePythonCommentMask(lines []string) []bool {
	mask := make([]bool, len(lines))
	inTripleSingle := false
	inTripleDouble := false

	for i := 0; i < len(lines); i++ {
		line := lines[i]

		if inTripleSingle || inTripleDouble {
			mask[i] = true
			if inTripleSingle && strings.Contains(line, "'''") {
				// 检查是否结束（简单处理：只检查是否出现结束标记）
				// 但可能在同一行开始和结束，需要更精细处理
				idx := strings.Index(line, "'''")
				if idx >= 0 {
					remaining := line[idx+3:]
					if !strings.Contains(remaining, "'''") {
						inTripleSingle = false
					}
				}
			}
			if inTripleDouble && strings.Contains(line, `"""`) {
				idx := strings.Index(line, `"""`)
				if idx >= 0 {
					remaining := line[idx+3:]
					if !strings.Contains(remaining, `"""`) {
						inTripleDouble = false
					}
				}
			}
			continue
		}

		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "#") {
			mask[i] = true
			continue
		}

		// 检查多行字符串开始
		if strings.Contains(line, `"""`) {
			// 简单启发：如果出现奇数个 """
			count := strings.Count(line, `"""`)
			if count%2 == 1 {
				inTripleDouble = true
				mask[i] = true
				continue
			}
		}
		if strings.Contains(line, "'''") {
			count := strings.Count(line, "'''")
			if count%2 == 1 {
				inTripleSingle = true
				mask[i] = true
				continue
			}
		}
	}

	return mask
}

// makePythonStringMask 标记 Python 单行字符串（单引号/双引号内的行）
// 注意：三引号字符串已在 makePythonCommentMask 中处理，
// 但单行 "..." 和 '...' 中的函数调用（如 f"{x}"）需要被屏蔽，避免假阳性。
func makePythonStringMask(lines []string) []bool {
	mask := make([]bool, len(lines))
	for i, line := range lines {
		inDouble := false
		inSingle := false
		escaped := false
		for _, ch := range line {
			if escaped {
				escaped = false
				continue
			}
			if ch == '\\' {
				escaped = true
				continue
			}
			if ch == '"' && !inSingle {
				inDouble = !inDouble
			} else if ch == '\'' && !inDouble {
				inSingle = !inSingle
			}
		}
		// 如果行结束时空引号未闭合，说明这是跨行字符串（Python 中很少见，
		// 多行字符串通常用三引号），按未闭合标记
		mask[i] = inDouble || inSingle
	}
	return mask
}

// extractPythonCalls 提取 Python 函数调用（委托给通用提取器）
func extractPythonCalls(body string, callRegex *regexp.Regexp) []string {
	return extractCallsSimple(body, callRegex, func(name string) bool {
		return isPyKeyword(name) || name[0] == '_'
	})
}

// pyKeywords 是 Python 关键字和常用标准库名的集合（map 数据驱动）
var pyKeywords = map[string]bool{
	"if": true, "elif": true, "else": true, "for": true, "while": true, "with": true, "as": true,
	"try": true, "except": true, "finally": true, "raise": true, "return": true, "yield": true,
	"import": true, "from": true, "class": true, "def": true, "lambda": true, "pass": true,
	"break": true, "continue": true, "and": true, "or": true, "not": true, "in": true, "is": true,
	"True": true, "False": true, "None": true, "self": true, "cls": true, "super": true,
	"print": true, "len": true, "range": true, "int": true, "str": true, "float": true, "list": true,
	"dict": true, "set": true, "tuple": true, "type": true,
	"isinstance": true, "hasattr": true, "getattr": true, "setattr": true, "delattr": true,
	"open": true, "zip": true, "map": true, "filter": true, "sorted": true, "reversed": true,
	"enumerate": true, "iter": true, "next": true, "input": true, "format": true,
	"abs": true, "all": true, "any": true, "bin": true, "bool": true, "bytearray": true, "bytes": true,
	"callable": true, "chr": true, "classmethod": true, "compile": true, "complex": true,
	"dir": true, "divmod": true, "eval": true, "exec": true, "frozenset": true,
	"globals": true, "hash": true, "hex": true, "id": true, "issubclass": true,
	"locals": true, "max": true, "memoryview": true, "min": true, "object": true,
	"oct": true, "ord": true, "pow": true, "property": true, "repr": true,
	"round": true, "slice": true, "staticmethod": true, "sum": true, "vars": true,
	"__import__": true,
	// os 模块
	"abort": true, "chdir": true, "chmod": true, "chown": true, "cpu_count": true, "ctermid": true,
	"environ": true, "getcwd": true, "getenv": true, "getpid": true, "getppid": true,
	"kill": true, "listdir": true, "makedirs": true, "mkdir": true, "path": true,
	"readlink": true, "remove": true, "removedirs": true, "rename": true, "replace": true,
	"rmdir": true, "stat": true, "symlink": true, "system": true, "uname": true, "unlink": true, "walk": true,
	"putenv": true, "unsetenv": true, "get_exec_path": true, "confstr": true, "fsencode": true, "fsdecode": true,
	"fstat": true, "lstat": true, "major": true, "minor": true, "makedev": true, "urandom": true,
	"sep": true, "linesep": true, "pathsep": true, "curdir": true, "pardir": true, "extsep": true, "altsep": true, "devnull": true,
	"name": true, "O_APPEND": true, "O_CREAT": true, "O_EXCL": true, "O_RDONLY": true, "O_RDWR": true, "O_WRONLY": true, "O_TRUNC": true,
	// os.path 模块
	"join": true, "splitext": true, "basename": true, "dirname": true,
	"exists": true, "isfile": true, "isdir": true, "islink": true, "ismount": true, "isabs": true,
	"abspath": true, "realpath": true, "relpath": true, "commonpath": true, "commonprefix": true,
	"normpath": true, "normcase": true, "expanduser": true, "expandvars": true,
	"getsize": true, "getmtime": true, "getatime": true, "getctime": true,
	"samefile": true, "sameopenfile": true, "samestat": true, "supports_unicode_filenames": true,
	// json 模块
	"dump": true, "dumps": true, "load": true, "loads": true, "JSONDecoder": true, "JSONEncoder": true,
	// re 模块
	"search": true, "match": true, "findall": true, "finditer": true,
	"split": true, "sub": true, "subn": true, "fullmatch": true, "escape": true,
	"purge": true, "template": true,
	"IGNORECASE": true, "I": true, "MULTILINE": true, "M": true, "DOTALL": true, "S": true,
	"VERBOSE": true, "X": true, "UNICODE": true, "U": true, "ASCII": true, "A": true, "LOCALE": true, "L": true, "DEBUG": true, "NOFLAG": true,
	// datetime 模块
	"now": true, "today": true, "utcnow": true, "fromtimestamp": true, "strftime": true, "strptime": true,
	"timedelta": true, "time": true, "date": true, "datetime": true,
	"timezone": true, "tzinfo": true, "minyear": true, "maxyear": true,
	"combine": true, "fromordinal": true, "fromisoformat": true, "fromisocalendar": true, "isocalendar": true, "isoformat": true,
	"isoweekday": true, "weekday": true, "astimezone": true, "utcoffset": true, "dst": true, "tzname": true,
	// math 模块
	"ceil": true, "comb": true, "copysign": true, "fabs": true, "factorial": true, "floor": true,
	"fmod": true, "frexp": true, "fsum": true, "gcd": true, "isclose": true, "isfinite": true,
	"isinf": true, "isnan": true, "isqrt": true, "ldexp": true, "modf": true, "nextafter": true,
	"perm": true, "prod": true, "remainder": true, "trunc": true,
	"exp": true, "log": true, "log2": true, "log10": true, "log1p": true, "expm1": true, "exp2": true,
	"sqrt": true, "cbrt": true, "hypot": true, "dist": true,
	"cos": true, "sin": true, "tan": true, "acos": true, "asin": true, "atan": true,
	"atan2": true, "cosh": true, "sinh": true, "tanh": true, "acosh": true, "asinh": true, "atanh": true,
	"degrees": true, "radians": true, "pi": true, "e": true, "tau": true, "inf": true, "nan": true,
	// subprocess 模块
	"run": true, "call": true, "check_call": true, "check_output": true, "Popen": true,
	"PIPE": true, "STDOUT": true, "DEVNULL": true, "CalledProcessError": true, "TimeoutExpired": true,
	"CompletedProcess": true, "SubprocessError": true, "STARTUPINFO": true, "STARTF_USESHOWWINDOW": true,
	// sys 模块
	"argv": true, "exit": true, "getdefaultencoding": true, "getrecursionlimit": true,
	"platform": true, "setrecursionlimit": true, "stdin": true, "stdout": true, "stderr": true,
	"version": true, "version_info": true,
	"modules": true, "meta_path": true, "path_hooks": true, "path_importer_cache": true,
	"prefix": true, "exec_prefix": true, "executable": true, "dllhandle": true, "winver": true,
	"api_version": true, "copyright": true, "builtin_module_names": true,
	"maxsize": true, "maxunicode": true, "float_info": true, "hash_info": true, "int_info": true,
	"getsizeof": true, "getrefcount": true, "getframe": true, "_current_frames": true,
	"exc_info": true, "settrace": true, "setprofile": true, "call_tracing": true,
	"audit": true, "addaudithook": true,
	// collections 模块
	"defaultdict": true, "Counter": true, "deque": true, "namedtuple": true, "OrderedDict": true,
	"ChainMap": true, "UserDict": true, "UserList": true, "UserString": true,
	"Collection": true, "KeysView": true, "ValuesView": true, "ItemsView": true,
	// itertools 模块
	"chain": true, "cycle": true, "repeat": true, "accumulate": true, "product": true,
	"permutations": true, "combinations": true, "combinations_with_replacement": true,
	"groupby": true, "islice": true, "starmap": true, "takewhile": true, "dropwhile": true,
	"filterfalse": true, "compress": true, "count": true, "zip_longest": true, "tee": true,
	"pairwise": true, "batched": true,
	// functools 模块
	"partial": true, "reduce": true, "wraps": true, "lru_cache": true, "cache": true,
	"cached_property": true, "singledispatch": true, "singledispatchmethod": true,
	"total_ordering": true, "cmp_to_key": true, "partialmethod": true,
	"update_wrapper": true,
	// random 模块
	"randint": true, "choice": true, "shuffle": true, "sample": true, "uniform": true,
	"seed": true, "random": true, "randrange": true, "gauss": true, "triangular": true,
	"betavariate": true, "expovariate": true, "gammavariate": true, "lognormvariate": true,
	"normalvariate": true, "vonmisesvariate": true, "paretovariate": true, "weibullvariate": true,
	"getstate": true, "setstate": true, "getrandbits": true, "SystemRandom": true,
	// statistics 模块
	"mean": true, "median": true, "median_low": true, "median_high": true, "median_grouped": true,
	"mode": true, "multimode": true, "stdev": true, "pstdev": true, "variance": true, "pvariance": true,
	"harmonic_mean": true, "geometric_mean": true, "quantiles": true, "NormalDist": true, "correlation": true, "linear_regression": true,
	// typing 模块
	"List": true, "Dict": true, "Tuple": true, "Set": true, "FrozenSet": true, "Optional": true,
	"Union": true, "Any": true, "Callable": true, "TypeVar": true, "Generic": true, "Protocol": true,
	"Iterator": true, "Iterable": true, "Sequence": true, "Mapping": true, "MutableMapping": true,
	"MutableSequence": true, "AnyStr": true, "cast": true, "NewType": true, "Type": true,
	"Literal": true, "Final": true, "ClassVar": true, "NamedTuple": true, "TypedDict": true,
	"overload": true, "TypeGuard": true, "ParamSpec": true, "Self": true,
	// pathlib 模块
	"Path": true, "PurePath": true, "PurePosixPath": true, "PureWindowsPath": true,
	"PosixPath": true, "WindowsPath": true,
	// shutil 模块
	"copy2": true, "copymode": true, "copystat": true, "copyfile": true,
	"copyfileobj": true, "copytree": true, "rmtree": true, "move": true,
	"make_archive": true, "get_archive_formats": true, "get_unpack_formats": true,
	"unpack_archive": true, "disk_usage": true, "which": true,
	"ignore_patterns": true, "SpecialFileError": true, "Error": true,
	// glob 模块
	"glob": true, "iglob": true,
	// tempfile 模块
	"TemporaryFile": true, "NamedTemporaryFile": true, "TemporaryDirectory": true,
	"mkstemp": true, "mkdtemp": true, "mktemp": true, "SpooledTemporaryFile": true,
	"gettempdir": true, "gettempprefix": true, "tempdir": true,
	// io 模块
	"StringIO": true, "BytesIO": true, "TextIOWrapper": true, "BufferedReader": true,
	"BufferedWriter": true, "BufferedRandom": true, "IOBase": true,
	// base64 模块
	"b64encode": true, "b64decode": true, "urlsafe_b64encode": true, "urlsafe_b64decode": true,
	"standard_b64encode": true, "standard_b64decode": true,
	"b32encode": true, "b32decode": true, "b16encode": true, "b16decode": true,
	// hashlib 模块
	"md5": true, "sha1": true, "sha224": true, "sha256": true, "sha384": true, "sha512": true,
	"sha3_224": true, "sha3_256": true, "sha3_384": true, "sha3_512": true,
	"shake_128": true, "shake_256": true, "blake2b": true, "blake2s": true,
	"pbkdf2_hmac": true, "new": true, "algorithms_available": true, "algorithms_guaranteed": true,
	// hmac 模块
	"hmac": true, "compare_digest": true, "digest": true, "hexdigest": true,
	// csv 模块
	"reader": true, "writer": true, "DictReader": true, "DictWriter": true,
	"Sniffer": true, "excel": true, "excel_tab": true, "unix_dialect": true,
	"QUOTE_ALL": true, "QUOTE_MINIMAL": true, "QUOTE_NONE": true, "QUOTE_NONNUMERIC": true,
	// configparser 模块
	"ConfigParser": true, "RawConfigParser": true, "ExtendedInterpolation": true, "BasicInterpolation": true,
	"ParsingError": true, "MissingSectionHeaderError": true, "DuplicateSectionError": true, "DuplicateOptionError": true,
	"InterpolationError": true, "InterpolationMissingOptionError": true, "InterpolationSyntaxError": true, "InterpolationDepthError": true,
	// logging 模块
	"getLogger": true, "basicConfig": true, "info": true, "debug": true,
	"warning": true, "critical": true, "exception": true,
	"Logger": true, "LoggerAdapter": true, "Handler": true, "StreamHandler": true,
	"FileHandler": true, "RotatingFileHandler": true, "TimedRotatingFileHandler": true,
	"NullHandler": true, "Formatter": true, "Filter": true,
	"INFO": true, "WARNING": true, "ERROR": true, "CRITICAL": true, "NOTSET": true,
	"addLevelName": true, "getLevelName": true, "makeLogRecord": true, "setLoggerClass": true, "shutdown": true,
	"captureWarnings": true,
	// traceback 模块
	"format_exc": true, "print_exc": true, "extract_tb": true, "print_tb": true,
	"format_tb": true, "format_list": true, "extract_stack": true, "print_stack": true,
	"format_stack": true, "format_exception": true, "print_exception": true, "clear_frames": true,
	// smtplib 模块
	"SMTP": true, "SMTP_SSL": true, "SMTPException": true, "SMTPServerDisconnected": true,
	"SMTPSenderRefused": true, "SMTPRecipientsRefused": true, "SMTPResponseException": true,
	"SMTPConnectError": true, "SMTPHeloError": true, "SMTPNotSupportedError": true, "SMTPAuthenticationError": true,
	// urllib 模块
	"parse": true, "robotparser": true,
	"urlopen": true, "urlretrieve": true, "urlcleanup": true, "Request": true, "urlencode": true,
	"quote": true, "quote_plus": true, "unquote": true, "unquote_plus": true, "urlsplit": true, "urlunsplit": true,
	"urljoin": true, "urlparse": true, "urlunparse": true, "parse_qs": true, "parse_qsl": true,
	"URLError": true, "HTTPError": true, "ContentTooShortError": true,
	// threading 模块
	"Thread": true, "Lock": true, "RLock": true, "Semaphore": true, "BoundedSemaphore": true,
	"Event": true, "Condition": true, "Barrier": true, "Timer": true, "local": true,
	"current_thread": true, "main_thread": true, "active_count": true,
	"stack_size": true, "TIMEOUT_MAX": true,
	// multiprocessing 模块
	"Process": true, "Pool": true, "Queue": true, "Pipe": true, "connection": true,
	"Value": true, "Array": true, "Manager": true,
	"current_process": true, "active_children": true,
	"freeze_support": true, "set_start_method": true, "get_context": true, "get_start_method": true,
	"Listener": true, "Client": true,
	// asyncio 模块
	"sleep": true, "gather": true, "wait": true, "wait_for": true,
	"as_completed": true, "create_task": true, "ensure_future": true, "shield": true,
	"Future": true, "Task": true, "CancelledError": true, "InvalidStateError": true, "TimeoutError": true,
	"run_coroutine_threadsafe": true, "all_tasks": true, "current_task": true,
	"get_event_loop": true, "new_event_loop": true, "set_event_loop": true, "get_running_loop": true,
	"run_until_complete": true, "wrap_future": true, "iscoroutine": true, "iscoroutinefunction": true,
	"isfuture": true, "isgenerator": true, "isgeneratorfunction": true,
	// socket 模块
	"socket": true, "create_connection": true, "create_server": true,
	"getaddrinfo": true, "gethostbyname": true, "gethostname": true,
	"getprotobyname": true, "getservbyname": true, "getservbyport": true,
	"ntohl": true, "htonl": true, "ntohs": true, "htons": true,
	"inet_aton": true, "inet_ntoa": true, "inet_pton": true, "inet_ntop": true,
	"AF_INET": true, "AF_INET6": true, "AF_UNIX": true, "SOCK_STREAM": true, "SOCK_DGRAM": true, "SOCK_RAW": true,
	"SHUT_RD": true, "SHUT_WR": true, "SHUT_RDWR": true, "SOL_SOCKET": true, "SO_REUSEADDR": true, "SO_REUSEPORT": true,
	"IPPROTO_TCP": true, "IPPROTO_UDP": true, "TCP_NODELAY": true, "timeout": true, "error": true, "herror": true, "gaierror": true,
	// struct 模块
	"pack": true, "unpack": true, "pack_into": true, "unpack_from": true,
	"calcsize": true, "Struct": true, "iter_unpack": true,
	// pickle 模块
	"HIGHEST_PROTOCOL": true, "DEFAULT_PROTOCOL": true, "Pickler": true, "Unpickler": true,
	"PicklingError": true, "UnpicklingError": true,
	// shelve 模块
	// sqlite3 模块
	"connect": true, "Connection": true, "Cursor": true, "Row": true,
	"OperationalError": true, "IntegrityError": true, "ProgrammingError": true,
	"InterfaceError": true, "DatabaseError": true, "DataError": true,
	"NotSupportedError": true, "Warning": true,
	"paramstyle": true, "sqlite_version": true, "sqlite_version_info": true,
	"register_adapter": true, "register_converter": true, "adapt": true, "complete_statement": true,
	"enable_shared_cache": true,
	// decimal 模块
	"Decimal": true, "getcontext": true, "setcontext": true, "localcontext": true,
	"BasicContext": true, "ExtendedContext": true, "DefaultContext": true,
	"DecimalException": true, "Clamped": true, "Rounded": true, "Inexact": true,
	"Subnormal": true, "Underflow": true, "Overflow": true, "DivisionByZero": true,
	"DivisionImpossible": true, "DivisionUndefined": true, "InvalidOperation": true,
	"ROUND_CEILING": true, "ROUND_DOWN": true, "ROUND_FLOOR": true,
	"ROUND_HALF_DOWN": true, "ROUND_HALF_EVEN": true, "ROUND_HALF_UP": true,
	"ROUND_UP": true, "ROUND_05UP": true, "MAX_PREC": true, "MAX_EMAX": true,
	// fractions 模块
	"Fraction": true,
	// enum 模块
	"Enum": true, "IntEnum": true, "Flag": true, "IntFlag": true, "auto": true, "unique": true, "EnumMeta": true,
	// dataclasses 模块
	"dataclass": true, "field": true, "asdict": true, "astuple": true,
	"FrozenInstanceError": true, "InitVar": true, "MISSING": true, "Field": true,
	// abc 模块
	"ABC": true, "abstractmethod": true, "abstractproperty": true, "abstractstaticmethod": true,
	"abstractclassmethod": true, "ABCMeta": true,
	// contextlib 模块
	"contextmanager": true, "suppress": true, "closing": true, "nullcontext": true,
	"ExitStack": true, "ContextDecorator": true, "redirect_stdout": true, "redirect_stderr": true,
	"AbstractContextManager": true, "AbstractAsyncContextManager": true,
	"asynccontextmanager": true, "aclosing": true, "AsyncExitStack": true,
	// copy 模块
	"deepcopy": true,
	// textwrap 模块
	"wrap": true, "fill": true, "dedent": true, "indent": true, "shorten": true, "TextWrapper": true,
	// string 模块
	"Template": true, "ascii_letters": true, "ascii_lowercase": true, "ascii_uppercase": true,
	"digits": true, "hexdigits": true, "octdigits": true, "punctuation": true,
	"printable": true, "whitespace": true, "capwords": true,
	// time 模块
	"localtime": true, "gmtime": true,
	"mktime": true, "monotonic": true, "perf_counter": true, "process_time": true, "thread_time": true,
	"ctime": true, "asctime": true, "clock_gettime": true, "clock_getres": true, "clock_settime": true,
	"CLOCK_MONOTONIC": true, "CLOCK_REALTIME": true, "CLOCK_PROCESS_CPUTIME_ID": true, "CLOCK_THREAD_CPUTIME_ID": true,
	"altzone": true, "daylight": true,
	// 常用第三方库 — requests
	"get": true, "post": true, "put": true, "delete": true, "patch": true, "head": true, "options": true,
	"Session": true, "Response": true, "request": true,
	// 常用第三方库 — numpy
	"array": true, "zeros": true, "ones": true, "arange": true, "linspace": true, "logspace": true,
	"reshape": true, "shape": true, "ndarray": true, "dot": true, "std": true, "var": true,
	"concatenate": true, "transpose": true, "sort": true, "argsort": true, "where": true,
	"expand_dims": true, "squeeze": true, "stack": true, "hstack": true, "vstack": true, "column_stack": true,
	"rand": true, "randn": true,
	"argmin": true, "argmax": true,
	// 常用第三方库 — pandas
	"DataFrame": true, "Series": true, "read_csv": true, "read_excel": true, "read_json": true,
	"read_sql": true, "read_sql_query": true, "read_sql_table": true, "read_parquet": true,
	"read_hdf": true, "read_feather": true, "read_stata": true, "read_sas": true,
	"read_pickle": true, "read_html": true, "read_clipboard": true,
	"concat": true, "merge": true, "pivot_table": true, "crosstab": true,
	"cut": true, "qcut": true, "get_dummies": true, "factorize": true,
	"date_range": true, "bdate_range": true, "period_range": true, "timedelta_range": true,
	"Timestamp": true, "Timedelta": true, "Interval": true, "IntervalIndex": true,
	"Period": true, "PeriodIndex": true, "NaT": true, "NA": true,
	"isna": true, "notna": true, "to_numeric": true, "to_datetime": true, "to_timedelta": true,
	"pivot": true, "melt": true, "unstack": true,
	// 常用第三方库 — flask
	"Flask": true, "jsonify": true, "render_template": true, "redirect": true,
	"url_for": true, "make_response": true, "send_file": true, "send_from_directory": true,
	"session": true, "flash": true, "Markup": true, "Blueprint": true,
	"g": true, "has_request_context": true,
	// 常用第三方库 — django
	"HttpResponse": true, "JsonResponse": true, "render": true, "reverse": true,
	"get_object_or_404": true, "get_list_or_404": true, "Http404": true,
	"HttpResponseRedirect": true, "HttpResponsePermanentRedirect": true,
	"HttpResponseNotAllowed": true, "HttpResponseBadRequest": true, "HttpResponseForbidden": true,
	"render_to_response": true, "StreamingHttpResponse": true, "FileResponse": true,
	// 常用第三方库 — pytest
	"fixture": true, "mark": true, "raises": true, "approx": true, "param": true,
	"skip": true, "skipif": true, "xfail": true, "importorskip": true, "fail": true,
	"yield_fixture": true, "config": true, "register_assert_rewrite": true,
	// 常用第三方库 — sqlalchemy
	"create_engine": true, "engine_from_config": true,
	"Column": true, "Integer": true, "String": true, "Float": true, "Boolean": true, "DateTime": true,
	"Text": true, "UnicodeText": true, "LargeBinary": true,
	"ForeignKey": true, "ForeignKeyConstraint": true, "PrimaryKeyConstraint": true,
	"UniqueConstraint": true, "CheckConstraint": true, "relationship": true, "backref": true,
	"declarative_base": true, "sessionmaker": true, "scoped_session": true, "aliased": true,
	"and_": true, "or_": true, "not_": true, "desc": true, "asc": true, "func": true,
	"text": true,
	"bindparam": true, "literal_column": true, "case": true, "type_coerce": true,
	"extract": true, "between": true, "in_": true, "notin_": true, "like": true, "ilike": true,
	"startswith": true, "endswith": true, "contains": true,
	"outerjoin": true, "Query": true, "Table": true, "MetaData": true,
	// 常用第三方库 — celery
	"Celery": true, "task": true, "group": true, "chord": true, "signature": true,
	"canvas": true, "subtask": true, "AsyncResult": true, "ResultSet": true,
	"shared_task": true, "states": true,
}

// isPyKeyword 过滤 Python 关键字
func isPyKeyword(name string) bool {
	return pyKeywords[name]
}

// countIndent 计算行的缩进级别（按空格计）
func countIndent(line string) int {
	count := 0
	for _, ch := range line {
		if ch == ' ' {
			count++
		} else if ch == '\t' {
			count += 4 // tab 视为 4 空格
		} else {
			break
		}
	}
	return count
}
