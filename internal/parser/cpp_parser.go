package parser

import (
	"regexp"
	"strings"
	"unicode"

	"code-detector/internal/model"
)

// CPPParser 解析 C/C++ 源文件
type CPPParser struct{}

func NewCPPParser() *CPPParser { return &CPPParser{} }
func (p *CPPParser) Language() string { return "cpp" }

func (p *CPPParser) Globals(filePath string, content []byte) ([]*model.GlobalVariable, error) {
	text := string(content)
	lines := strings.Split(text, "\n")
	commentMask := makeCommentMask(lines, []string{"//"}, [][2]string{{"/*", "*/"}})
	preprocMask := makePreprocessorMask(lines)

	var vars []*model.GlobalVariable
	// 匹配文件作用域的变量声明: type name = value; 或 type name;
	// 排除函数定义、返回类型声明等
	cppVarRegex := regexp.MustCompile(`^\s*(?:extern\s+)?(?:(?P<type>(?:unsigned\s+)?(?:long\s+)?(?:short\s+)?(?:signed\s+)?\w+(?:\s*\*)?(?:\s+const)?))\s+(?P<name>\w+)\s*(?:=|;)`)

	for i, line := range lines {
		if commentMask[i] || preprocMask[i] {
			continue
		}
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.HasPrefix(trimmed, "//") || strings.HasPrefix(trimmed, "/*") {
			continue
		}
		if strings.HasPrefix(trimmed, "#") {
			continue
		}
		// 跳过函数定义、控制流
		if strings.HasPrefix(trimmed, "if ") || strings.HasPrefix(trimmed, "for ") ||
			strings.HasPrefix(trimmed, "while ") || strings.HasPrefix(trimmed, "switch ") {
			continue
		}
		matches := cppVarRegex.FindStringSubmatch(line)
		if matches == nil {
			continue
		}
		nameIdx := cppVarRegex.SubexpIndex("name")
		typeIdx := cppVarRegex.SubexpIndex("type")
		name := matches[nameIdx]
		varType := ""
		if typeIdx >= 0 && typeIdx < len(matches) {
			varType = matches[typeIdx]
		}
		if name != "" && varType != "" && !isCPPKeyword(name) && !isCPPKeyword(varType) && (unicode.IsLetter(rune(name[0])) || name[0] == '_') {
			// 提取 namespace 和可见性
			pkgName := extractCPPNamespace(lines, commentMask)
			visibility := "public"
			if strings.Contains(trimmed, "static ") || strings.HasPrefix(trimmed, "static") {
				visibility = "private"
			}
			vars = append(vars, &model.GlobalVariable{
				Name: name, VarType: varType, Language: "cpp",
				PackageName: pkgName, Visibility: visibility,
				FilePath: filePath, LineNum: i + 1,
			})
		}
	}
	return vars, nil
}

// cppNamespaceRegex 匹配 C++ namespace 声明
var cppNamespaceRegex = regexp.MustCompile(`^\s*namespace\s+(?P<name>\w+)`)

// extractCPPNamespace 提取 C++ 文件中的第一个 namespace
func extractCPPNamespace(lines []string, commentMask []bool) string {
	for i, line := range lines {
		if commentMask[i] {
			continue
		}
		matches := cppNamespaceRegex.FindStringSubmatch(line)
		if matches != nil {
			nameIdx := cppNamespaceRegex.SubexpIndex("name")
			if nameIdx >= 0 && nameIdx < len(matches) {
				return matches[nameIdx]
			}
		}
	}
	return ""
}

var cppFuncRegex = regexp.MustCompile(
	`(?:(?:\w+(?:\[\])*(?:\s*<[^>]+>)?\s+)+)(?P<name>\w+)\s*\(`,
)

var cppCallRegex = regexp.MustCompile(`(?:(\w+)::)?(\w+)\s*\(`)

func (p *CPPParser) Parse(filePath string, content []byte) ([]*model.Function, error) {
	fl := NewFileLinesFromBytes(content)
	lines := fl.Lines()
	text := fl.Text()

	commentMask := makeCommentMask(lines, []string{"//"}, [][2]string{{"/*", "*/"}})
	stringMask := makeStringMask(lines)
	// 预处理掩码：跳过 #if 0 死代码、#define 宏定义体、#include 等
	preprocMask := makePreprocessorMask(lines)

	type cppFuncStart struct {
		lineIdx int
		name    string
	}
	var starts []cppFuncStart

	for i, line := range lines {
		if commentMask[i] || stringMask[i] || preprocMask[i] {
			continue
		}
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			continue
		}
		// 排除 # 预处理指令（未被子掩码覆盖的）
		if strings.HasPrefix(trimmed, "#") {
			continue
		}

		matches := cppFuncRegex.FindStringSubmatch(line)
		if matches == nil {
			continue
		}
		nameIdx := cppFuncRegex.SubexpIndex("name")
		if nameIdx < 0 || nameIdx >= len(matches) {
			continue
		}
		name := matches[nameIdx]
		if name == "" || isCPPKeyword(name) {
			continue
		}

		starts = append(starts, cppFuncStart{lineIdx: i, name: name})
	}

	if len(starts) == 0 {
		return nil, nil
	}

	allFuncs := make([]*model.Function, 0, len(starts))
	for _, fs := range starts {
		braceOffset := -1
		for j := fs.lineIdx; j < len(lines); j++ {
			line := lines[j]
			idx := findBraceInLine(line)
			if idx >= 0 && !commentMask[j] && !preprocMask[j] {
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
		endLine := fl.LineFromOffset( closeOffset)

		bodyStart := fl.LineOffset( startLine)
		bodyEnd := closeOffset + 1
		if bodyEnd > len(text) {
			bodyEnd = len(text)
		}
		body := text[bodyStart:bodyEnd]

		callStats := extractCallStats(body, cppCallRegex, stringMask, commentMask, startLine, endLine, isCPPKeyword, nil)

		f := &model.Function{
			Name:         fs.name,
			PackageName:  extractCPPNamespace(lines, commentMask),
			Language:     "cpp",
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

// cppKeywords 是 C/C++ 关键字和常用标准库名的集合（map 数据驱动）
var cppKeywords = map[string]bool{
	// C/C++ 语言关键字
	"if": true, "else": true, "for": true, "while": true, "do": true, "switch": true, "case": true,
	"return": true, "break": true, "continue": true, "goto": true, "default": true,
	"try": true, "catch": true, "throw": true, "new": true, "delete": true, "this": true,
	"int": true, "long": true, "short": true, "char": true, "float": true, "double": true,
	"void": true, "bool": true, "signed": true, "unsigned": true, "const": true, "static": true,
	"virtual": true, "explicit": true, "inline": true, "typedef": true, "struct": true,
	"class": true, "enum": true, "union": true, "namespace": true, "using": true,
	"public": true, "private": true, "protected": true, "template": true, "typename": true,
	"sizeof": true, "nullptr": true, "true": true, "false": true, "auto": true, "register": true,
	"volatile": true, "mutable": true, "friend": true, "override": true, "final": true,
	"constexpr": true, "noexcept": true, "decltype": true, "nullptr_t": true,
	"export": true, "extern": true, "asm": true,
	// C99/C11/C23 关键字和类型
	"_Alignas": true, "_Alignof": true, "_Atomic": true, "_Bool": true, "_Complex": true,
	"_Generic": true, "_Imaginary": true, "_Noreturn": true, "_Static_assert": true,
	"_Thread_local": true, "alignas": true, "alignof": true, "atomic": true, "complex": true,
	"generic": true, "noreturn": true, "static_assert": true, "thread_local": true,
	"restrict": true,
	// C++11/14/17/20 新增关键字
	"consteval": true, "constinit": true, "concept": true, "requires": true,
	"co_await": true, "co_return": true, "co_yield": true, "char8_t": true, "char16_t": true, "char32_t": true,
	"wchar_t": true,
	// C 标准库 — stdio.h / cstdio
	"printf": true, "sprintf": true, "snprintf": true, "fprintf": true, "dprintf": true,
	"scanf": true, "sscanf": true, "fscanf": true, "vprintf": true, "vsprintf": true,
	"vsnprintf": true, "vfprintf": true, "vscanf": true, "vsscanf": true, "vfscanf": true,
	"fopen": true, "fclose": true, "fread": true, "fwrite": true, "fgetc": true, "fputc": true,
	"fgets": true, "fputs": true, "ungetc": true, "getc": true, "putc": true,
	"getchar": true, "putchar": true, "gets": true, "puts": true,
	"feof": true, "ferror": true, "clearerr": true, "perror": true,
	"remove": true, "rename": true, "tmpfile": true, "tmpnam": true,
	"setbuf": true, "setvbuf": true, "fflush": true, "rewind": true,
	"fseek": true, "ftell": true, "fgetpos": true, "fsetpos": true,
	"freopen": true, "fdopen": true, "fileno": true,
	// C 标准库 — stdlib.h / cstdlib
	"malloc": true, "calloc": true, "realloc": true, "free": true, "aligned_alloc": true,
	"atoi": true, "atol": true, "atoll": true, "atof": true,
	"strtol": true, "strtoll": true, "strtoul": true, "strtoull": true,
	"strtof": true, "strtod": true, "strtold": true,
	"rand": true, "srand": true, "RAND_MAX": true,
	"abs": true, "labs": true, "llabs": true, "div": true, "ldiv": true, "lldiv": true,
	"qsort": true, "bsearch": true, "exit": true, "_Exit": true, "atexit": true, "abort": true,
	"getenv": true, "system": true, "mkstemp": true, "mktemp": true,
	// C 标准库 — string.h / cstring
	"strlen": true, "strcmp": true, "strncmp": true, "strcpy": true, "strncpy": true,
	"strcat": true, "strncat": true, "strchr": true, "strrchr": true,
	"strstr": true, "strpbrk": true, "strspn": true, "strcspn": true,
	"strtok": true, "strtok_r": true, "strerror": true, "strerror_r": true,
	"memcpy": true, "memmove": true, "memcmp": true, "memset": true, "memchr": true,
	"strcoll": true, "strxfrm": true, "strdup": true, "strndup": true,
	// C 标准库 — ctype.h / cctype
	"isalnum": true, "isalpha": true, "isblank": true, "iscntrl": true, "isdigit": true,
	"isgraph": true, "islower": true, "isprint": true, "ispunct": true, "isspace": true,
	"isupper": true, "isxdigit": true, "tolower": true, "toupper": true,
	// C 标准库 — math.h / cmath (补充)
	"fabs": true, "fabsf": true, "fabsl": true,
	"ceil": true, "ceilf": true, "ceill": true, "floor": true, "floorf": true, "floorl": true,
	"round": true, "roundf": true, "roundl": true, "trunc": true, "truncf": true, "truncl": true,
	"fma": true, "fmaf": true, "fmal": true, "fdim": true, "fdimf": true, "fdiml": true,
	"fmax": true, "fmaxf": true, "fmaxl": true, "fmin": true, "fminf": true, "fminl": true,
	"copysign": true, "copysignf": true, "copysignl": true, "nan": true, "nanf": true, "nanl": true,
	"nearbyint": true, "nearbyintf": true, "nearbyintl": true,
	"rint": true, "rintf": true, "rintl": true, "lrint": true, "lrintf": true, "lrintl": true,
	"llrint": true, "llrintf": true, "llrintl": true,
	"lround": true, "lroundf": true, "lroundl": true, "llround": true, "llroundf": true, "llroundl": true,
	"sqrt": true, "sqrtf": true, "sqrtl": true, "cbrt": true, "cbrtf": true, "cbrtl": true,
	"hypot": true, "hypotf": true, "hypotl": true, "pow": true, "powf": true, "powl": true,
	"exp": true, "expf": true, "expl": true, "exp2": true, "expm1": true,
	"log": true, "logf": true, "logl": true, "log2": true, "log2f": true, "log2l": true,
	"log10": true, "log10f": true, "log10l": true, "log1p": true, "logb": true, "ilogb": true,
	"sin": true, "sinf": true, "sinl": true, "cos": true, "cosf": true, "cosl": true,
	"tan": true, "tanf": true, "tanl": true, "asin": true, "asinf": true, "asinl": true,
	"acos": true, "acosf": true, "acosl": true, "atan": true, "atanf": true, "atanl": true,
	"atan2": true, "atan2f": true, "atan2l": true,
	"sinh": true, "sinhf": true, "sinhl": true, "cosh": true, "coshf": true, "coshl": true,
	"tanh": true, "tanhf": true, "tanhl": true, "asinh": true, "asinhf": true, "asinhl": true,
	"acosh": true, "acoshf": true, "acoshl": true, "atanh": true, "atanhf": true, "atanhl": true,
	"fmod": true, "fmodf": true, "fmodl": true, "remainder": true, "remainderf": true, "remainderl": true,
	"erf": true, "erff": true, "erfl": true, "erfc": true, "erfcf": true, "erfcl": true,
	"tgamma": true, "tgammaf": true, "tgammal": true, "lgamma": true, "lgammaf": true, "lgammal": true,
	"M_PI": true, "M_E": true, "M_LOG2E": true, "M_LOG10E": true, "M_LN2": true, "M_LN10": true,
	"M_PI_2": true, "M_PI_4": true, "M_1_PI": true, "M_2_PI": true, "M_2_SQRTPI": true,
	"M_SQRT2": true, "M_SQRT1_2": true, "HUGE_VAL": true, "HUGE_VALF": true, "HUGE_VALL": true,
	"INFINITY": true, "NAN": true, "math_errhandling": true,
	// C 标准库 — time.h / ctime
	"time": true, "clock": true, "difftime": true, "mktime": true,
	"asctime": true, "ctime": true, "gmtime": true, "localtime": true,
	"strftime": true, "timespec_get": true,
	"CLOCKS_PER_SEC": true, "TIME_UTC": true, "CLOCK_REALTIME": true, "CLOCK_MONOTONIC": true,
	"time_t": true, "clock_t": true, "timespec": true, "tm": true,
	// C 标准库 — signal.h / csignal
	"signal": true, "raise": true, "SIG_DFL": true, "SIG_IGN": true, "SIG_ERR": true,
	"SIGABRT": true, "SIGFPE": true, "SIGILL": true, "SIGINT": true, "SIGSEGV": true, "SIGTERM": true,
	// C 标准库 — setjmp.h / csetjmp
	"setjmp": true, "longjmp": true,
	// C 标准库 — stdarg.h / cstdarg
	"va_start": true, "va_end": true, "va_copy": true, "va_arg": true,
	// C 标准库 — stddef.h / cstddef
	"offsetof": true, "ptrdiff_t": true, "size_t": true, "max_align_t": true,
	// C 标准库 — stdint.h / cstdint
	"int8_t": true, "int16_t": true, "int32_t": true, "int64_t": true,
	"uint8_t": true, "uint16_t": true, "uint32_t": true, "uint64_t": true,
	"int_least8_t": true, "int_least16_t": true, "int_least32_t": true, "int_least64_t": true,
	"uint_least8_t": true, "uint_least16_t": true, "uint_least32_t": true, "uint_least64_t": true,
	"int_fast8_t": true, "int_fast16_t": true, "int_fast32_t": true, "int_fast64_t": true,
	"uint_fast8_t": true, "uint_fast16_t": true, "uint_fast32_t": true, "uint_fast64_t": true,
	"intptr_t": true, "uintptr_t": true, "intmax_t": true, "uintmax_t": true,
	"INT8_MIN": true, "INT16_MIN": true, "INT32_MIN": true, "INT64_MIN": true,
	"INT8_MAX": true, "INT16_MAX": true, "INT32_MAX": true, "INT64_MAX": true,
	"UINT8_MAX": true, "UINT16_MAX": true, "UINT32_MAX": true, "UINT64_MAX": true,
	// C 标准库 — limits.h / climits
	"CHAR_BIT": true, "SCHAR_MIN": true, "SCHAR_MAX": true, "UCHAR_MAX": true,
	"CHAR_MIN": true, "CHAR_MAX": true, "MB_LEN_MAX": true,
	"SHRT_MIN": true, "SHRT_MAX": true, "USHRT_MAX": true,
	"INT_MIN": true, "INT_MAX": true, "UINT_MAX": true,
	"LONG_MIN": true, "LONG_MAX": true, "ULONG_MAX": true,
	"LLONG_MIN": true, "LLONG_MAX": true, "ULLONG_MAX": true,
	// C 标准库 — float.h / cfloat
	"FLT_MIN": true, "FLT_MAX": true, "FLT_EPSILON": true, "FLT_DIG": true, "FLT_MANT_DIG": true,
	"DBL_MIN": true, "DBL_MAX": true, "DBL_EPSILON": true, "DBL_DIG": true, "DBL_MANT_DIG": true,
	"LDBL_MIN": true, "LDBL_MAX": true, "LDBL_EPSILON": true, "LDBL_DIG": true, "LDBL_MANT_DIG": true,
	// C 标准库 — errno.h / cerrno
	"errno": true, "EDOM": true, "ERANGE": true, "EILSEQ": true, "errno_t": true,
	// C 标准库 — assert.h / cassert
	"assert": true,
	// C 标准库 — locale.h / clocale
	"setlocale": true, "localeconv": true, "LC_ALL": true, "LC_COLLATE": true,
	"LC_CTYPE": true, "LC_MONETARY": true, "LC_NUMERIC": true, "LC_TIME": true,
	// C 标准库 — inttypes.h / cinttypes
	"imaxabs": true, "imaxdiv": true, "strtoimax": true, "strtoumax": true,
	"PRId8": true, "PRIi8": true, "PRIu8": true, "PRIx8": true, "PRIX8": true,
	"PRIi16": true, "PRIu16": true, "PRIx16": true,
	"PRIi32": true, "PRIu32": true, "PRIx32": true,
	"PRIi64": true, "PRIu64": true, "PRIx64": true, "SCNd8": true, "SCNu8": true, "SCNx8": true,
	// C++ STL — 容器 (补充)
	"vector": true, "list": true, "deque": true, "array": true, "forward_list": true,
	"map": true, "set": true, "multimap": true, "multiset": true,
	"unordered_map": true, "unordered_set": true,
	"unordered_multimap": true, "unordered_multiset": true,
	"stack": true, "queue": true, "priority_queue": true,
	"pair": true, "tuple": true, "optional": true, "variant": true, "any": true,
	"span": true, "string_view": true, "basic_string_view": true,
	"valarray": true, "slice": true, "gslice": true,
	"initializer_list": true,
	// C++ string / string_view
	"string": true, "wstring": true, "u16string": true, "u32string": true, "u8string": true,
	"getline": true, "to_string": true, "to_wstring": true,
	"stoi": true, "stol": true, "stoll": true, "stoul": true, "stoull": true,
	"stof": true, "stod": true, "stold": true,
	// C++ STL — 算法 (补充)
	"sort": true, "stable_sort": true, "partial_sort": true, "partial_sort_copy": true,
	"nth_element": true, "is_sorted": true, "is_sorted_until": true,
	"find": true, "find_if": true, "find_if_not": true, "find_end": true, "find_first_of": true,
	"adjacent_find": true, "search": true, "search_n": true,
	"binary_search": true, "lower_bound": true, "upper_bound": true, "equal_range": true,
	"count": true, "count_if": true, "copy": true, "copy_if": true, "copy_n": true, "copy_backward": true,
	"move": true, "move_backward": true, "fill": true, "fill_n": true,
	"generate": true, "generate_n": true, "transform": true,
	"replace": true, "replace_if": true, "replace_copy": true, "replace_copy_if": true,
	"remove_if": true, "remove_copy": true, "remove_copy_if": true,
	"unique": true, "unique_copy": true, "reverse": true, "reverse_copy": true,
	"rotate": true, "rotate_copy": true, "shuffle": true,
	"shift_left": true, "shift_right": true,
	"sample": true, "random_shuffle": true,
	"is_partitioned": true, "partition": true, "partition_copy": true, "partition_point": true,
	"stable_partition": true, "for_each": true, "for_each_n": true,
	"accumulate": true, "reduce": true, "transform_reduce": true,
	"inner_product": true, "adjacent_difference": true, "partial_sum": true,
	"inclusive_scan": true, "exclusive_scan": true, "transform_inclusive_scan": true,
	"transform_exclusive_scan": true,
	"max": true, "min": true, "max_element": true, "min_element": true, "clamp": true,
	"minmax": true, "minmax_element": true,
	"all_of": true, "any_of": true, "none_of": true, "equal": true, "mismatch": true,
	"lexicographical_compare": true, "lexicographical_compare_three_way": true,
	"merge": true, "inplace_merge": true, "includes": true,
	"set_union": true, "set_intersection": true, "set_difference": true, "set_symmetric_difference": true,
	"is_heap": true, "is_heap_until": true, "push_heap": true, "pop_heap": true,
	"make_heap": true, "sort_heap": true,
	"iota": true, "next_permutation": true, "prev_permutation": true, "is_permutation": true,
	"iter_swap": true, "swap_ranges": true,
	// C++ STL — I/O 流
	"cin": true, "cout": true, "cerr": true, "clog": true, "wcin": true, "wcout": true, "wcerr": true, "wclog": true,
	"endl": true, "ends": true, "flush": true, "ws": true, "hex": true, "dec": true, "oct": true,
	"setw": true, "setprecision": true, "setfill": true, "setbase": true, "setiosflags": true,
	"resetiosflags": true, "boolalpha": true, "noboolalpha": true, "showbase": true, "noshowbase": true,
	"showpoint": true, "noshowpoint": true, "showpos": true, "noshowpos": true,
	"skipws": true, "noskipws": true, "unitbuf": true, "nounitbuf": true,
	"uppercase": true, "nouppercase": true, "left": true, "right": true, "internal": true,
	"fixed": true, "scientific": true, "hexfloat": true, "defaultfloat": true,
	"ifstream": true, "ofstream": true, "fstream": true,
	"stringstream": true, "istringstream": true, "ostringstring": true,
	"stringbuf": true, "filebuf": true, "streambuf": true,
	"istream": true, "ostream": true, "iostream": true,
	// C++ STL — 智能指针 / memory
	"shared_ptr": true, "unique_ptr": true, "weak_ptr": true,
	"make_shared": true, "make_unique": true, "make_shared_for_overwrite": true,
	"allocate_shared": true, "dynamic_pointer_cast": true, "static_pointer_cast": true,
	"const_pointer_cast": true, "reinterpret_pointer_cast": true,
	"enable_shared_from_this": true, "bad_weak_ptr": true,
	"owner_less": true, "default_delete": true, "allocator": true, "allocator_traits": true,
	"addressof": true, "pointer_traits": true, "to_address": true,
	"construct_at": true, "destroy_at": true, "destroy": true, "destroy_n": true,
	"uninitialized_copy": true, "uninitialized_fill": true, "uninitialized_move": true,
	"uninitialized_default_construct": true, "uninitialized_value_construct": true,
	"raw_storage_iterator": true,
	// C++ STL — 并发 / thread
	"thread": true, "jthread": true, "mutex": true, "timed_mutex": true, "recursive_mutex": true,
	"recursive_timed_mutex": true, "shared_mutex": true, "shared_timed_mutex": true,
	"lock_guard": true, "unique_lock": true, "shared_lock": true, "scoped_lock": true,
	"condition_variable": true, "condition_variable_any": true,
	"notify_all_at_thread_exit": true,
	"future": true, "shared_future": true, "promise": true, "packaged_task": true,
	"async": true, "launch": true, "future_status": true,
	"this_thread": true, "sleep_for": true, "sleep_until": true, "yield": true, "get_id": true,
	"once_flag": true, "call_once": true,
	"atomic_flag": true, "ATOMIC_FLAG_INIT": true,
	"memory_order": true, "memory_order_relaxed": true, "memory_order_consume": true,
	"memory_order_acquire": true, "memory_order_release": true,
	"memory_order_acq_rel": true, "memory_order_seq_cst": true,
	"kill_dependency": true, "atomic_thread_fence": true, "atomic_signal_fence": true,
	// C++ STL — 迭代器 / iterator
	"back_inserter": true, "front_inserter": true, "inserter": true,
	"make_move_iterator": true, "make_reverse_iterator": true,
	"advance": true, "distance": true, "next": true, "prev": true,
	"begin": true, "end": true, "cbegin": true, "cend": true,
	"rbegin": true, "rend": true, "crbegin": true, "crend": true,
	"size": true, "empty": true, "data": true,
	"ssize": true, "ranges": true,
	// C++ STL — 函数对象 / functional
	"function": true, "bind": true, "ref": true, "cref": true, "hash": true,
	"plus": true, "minus": true, "multiplies": true, "divides": true, "modulus": true, "negate": true,
	"equal_to": true, "not_equal_to": true, "greater": true, "less": true,
	"greater_equal": true, "less_equal": true,
	"logical_and": true, "logical_or": true, "logical_not": true,
	"bit_and": true, "bit_or": true, "bit_xor": true, "bit_not": true,
	"identity": true, "ranges_equal_to": true, "ranges_not_equal_to": true,
	"default_searcher": true, "boyer_moore_searcher": true, "boyer_moore_horspool_searcher": true,
	"bad_function_call": true, "mem_fn": true, "invoke": true, "not_fn": true,
	// C++ STL — 类型工具 / type_traits
	"integral_constant": true, "true_type": true, "false_type": true,
	"is_same": true, "is_same_v": true, "is_void": true, "is_null_pointer": true,
	"is_integral": true, "is_floating_point": true, "is_array": true, "is_enum": true,
	"is_union": true, "is_class": true, "is_function": true, "is_pointer": true,
	"is_lvalue_reference": true, "is_rvalue_reference": true, "is_member_pointer": true,
	"is_const": true, "is_volatile": true, "is_trivial": true, "is_standard_layout": true,
	"is_pod": true, "is_literal_type": true, "is_empty": true, "is_polymorphic": true,
	"is_abstract": true, "is_final": true, "is_aggregate": true,
	"is_signed": true, "is_unsigned": true, "is_bounded_array": true, "is_unbounded_array": true,
	"is_scoped_enum": true,
	"is_constructible": true, "is_default_constructible": true, "is_copy_constructible": true,
	"is_move_constructible": true, "is_assignable": true, "is_copy_assignable": true,
	"is_move_assignable": true, "is_destructible": true, "is_swappable": true,
	"has_virtual_destructor": true, "alignment_of": true, "rank": true, "extent": true,
	"remove_const": true, "remove_volatile": true, "remove_cv": true, "add_const": true,
	"add_volatile": true, "add_cv": true, "remove_reference": true, "add_lvalue_reference": true,
	"add_rvalue_reference": true, "remove_pointer": true, "add_pointer": true,
	"make_signed": true, "make_unsigned": true, "remove_extent": true, "remove_all_extents": true,
	"decay": true, "remove_cvref": true, "conditional": true, "enable_if": true,
	"underlying_type": true, "invoke_result": true, "result_of": true,
	"common_type": true, "common_reference": true, "type_identity": true,
	"conjunction": true, "disjunction": true, "negation": true,
	"void_t": true,
	// C++ STL — utility / tuple
	"move_if_noexcept": true, "forward": true, "forward_like": true,
	"declval": true, "swap": true, "exchange": true, "as_const": true,
	"cmp_equal": true, "cmp_not_equal": true, "cmp_less": true, "cmp_greater": true,
	"cmp_less_equal": true, "cmp_greater_equal": true,
	"to_underlying": true, "unreachable": true,
	"integer_sequence": true, "make_integer_sequence": true, "index_sequence": true,
	"make_index_sequence": true, "index_sequence_for": true,
	"tuple_cat": true, "tuple_element": true, "tuple_size": true,
	"apply": true, "make_from_tuple": true,
	"type_info": true,
	"bitset": true,
	// C++ STL — 正则表达式 / regex
	"regex": true, "smatch": true, "cmatch": true, "ssub_match": true, "csub_match": true,
	"regex_match": true, "regex_search": true, "regex_replace": true,
	"regex_iterator": true, "regex_token_iterator": true, "regex_traits": true,
	"regex_constants": true, "icase": true, "nosubs": true, "optimize": true, "collate": true,
	"ECMAScript": true, "basic": true, "extended": true, "awk": true, "grep": true, "egrep": true,
	// C++ STL — 随机数 / random
	"mt19937": true, "mt19937_64": true, "minstd_rand0": true, "minstd_rand": true,
	"ranlux24_base": true, "ranlux48_base": true, "ranlux24": true, "ranlux48": true,
	"knuth_b": true, "default_random_engine": true,
	"random_device": true, "seed_seq": true,
	"uniform_int_distribution": true, "uniform_real_distribution": true,
	"normal_distribution": true, "lognormal_distribution": true, "gamma_distribution": true,
	"chi_squared_distribution": true, "cauchy_distribution": true, "fisher_f_distribution": true,
	"student_t_distribution": true, "bernoulli_distribution": true,
	"binomial_distribution": true, "negative_binomial_distribution": true,
	"geometric_distribution": true, "poisson_distribution": true,
	"exponential_distribution": true, "weibull_distribution": true,
	"extreme_value_distribution": true,
	"discrete_distribution": true, "piecewise_constant_distribution": true,
	"piecewise_linear_distribution": true,
	// C++ STL — 时间 / chrono
	"system_clock": true, "steady_clock": true, "high_resolution_clock": true,
	"duration": true, "nanoseconds": true, "microseconds": true,
	"milliseconds": true, "seconds": true, "minutes": true, "hours": true, "days": true, "weeks": true,
	"months": true, "years": true,
	"duration_cast": true, "time_point_cast": true,
	"time_point": true, "file_clock": true, "gps_clock": true, "tai_clock": true, "utc_clock": true,
	"sys_days": true, "sys_seconds": true, "local_t": true, "zoned_time": true,
	"year": true, "month": true, "day": true, "weekday": true, "year_month_day": true,
	"hh_mm_ss": true,
	// C++ STL — 文件系统 / filesystem
	"path": true, "filesystem": true, "directory_entry": true, "directory_iterator": true,
	"recursive_directory_iterator": true, "file_status": true, "file_type": true,
	"perms": true, "perm_options": true, "copy_options": true, "directory_options": true,
	"file_time_type": true, "space_info": true,
	"create_directory": true, "create_directories": true, "create_symlink": true,
	"copy_file": true, "copy_symlink": true,
	"current_path": true, "exists": true, "equivalent": true, "file_size": true,
	"hard_link_count": true, "is_block_file": true, "is_character_file": true,
	"is_directory": true, "is_fifo": true, "is_other": true,
	"is_regular_file": true, "is_socket": true, "is_symlink": true,
	"last_write_time": true, "permissions": true, "read_symlink": true,
	"absolute": true, "canonical": true, "weakly_canonical": true, "relative": true, "proximate": true,
	"remove_all": true, "resize_file": true,
	"space": true, "status": true, "status_known": true, "temp_directory_path": true,
	"u8path": true,
	// C++ 并行算法执行策略
	"execution": true, "seq": true, "par": true, "par_unseq": true, "unseq": true,
	// C++ 字符转换 / charconv
	"to_chars": true, "from_chars": true, "chars_format": true,
	"chars_format_scientific": true, "chars_format_fixed": true, "chars_format_hex": true,
	"chars_format_general": true,
	// C++ 位操作 / bit
	"bit_cast": true, "rotl": true, "rotr": true, "popcount": true, "countl_zero": true,
	"countr_zero": true, "countl_one": true, "countr_one": true, "bit_width": true,
	"bit_floor": true, "bit_ceil": true, "has_single_bit": true, "endian": true,
	// C++ 数值 / numeric
	"midpoint": true, "lerp": true, "gcd": true, "lcm": true,
	// C++ 比较 / compare
	"strong_ordering": true, "weak_ordering": true, "partial_ordering": true,
	"compare_three_way": true, "strong_order": true, "weak_order": true, "partial_order": true,
	"compare_strong_order_fallback": true, "compare_weak_order_fallback": true,
	"compare_partial_order_fallback": true, "is_eq": true, "is_neq": true, "is_lt": true,
	"is_lteq": true, "is_gt": true, "is_gteq": true,
	// C++ 概念 / concepts
	"same_as": true, "derived_from": true, "convertible_to": true,
	"common_reference_with": true, "common_with": true,
	"integral": true, "signed_integral": true, "unsigned_integral": true, "floating_point": true,
	"assignable_from": true, "swappable": true, "swappable_with": true,
	"destructible": true, "constructible_from": true, "default_initializable": true,
	"move_constructible": true, "copy_constructible": true,
	"equality_comparable": true, "totally_ordered": true,
	"regular": true, "semiregular": true, "invocable": true, "predicate": true,
	"relation": true, "strict_weak_order": true,
	// C++ 范围 / ranges
	"range": true, "sized_range": true, "view": true, "viewable_range": true,
	"input_range": true, "output_range": true, "forward_range": true,
	"bidirectional_range": true, "random_access_range": true, "contiguous_range": true,
	"common_range": true, "borrowed_range": true, "subrange": true,
	// POSIX / unistd
	"read": true, "write": true, "open": true, "close": true, "lseek": true,
	"pipe": true, "dup": true, "dup2": true,
	"fork": true, "exec": true, "execvp": true, "execlp": true, "execv": true, "execl": true,
	"execle": true, "execve": true, "execvpe": true,
	"wait": true, "waitpid": true, "waitid": true, "WNOHANG": true, "WUNTRACED": true,
	"WIFEXITED": true, "WEXITSTATUS": true, "WIFSIGNALED": true, "WTERMSIG": true,
	"WIFSTOPPED": true, "WSTOPSIG": true, "WIFCONTINUED": true,
	"getpid": true, "getppid": true, "getuid": true, "geteuid": true, "getgid": true, "getegid": true,
	"getgroups": true, "setuid": true, "seteuid": true, "setgid": true, "setegid": true,
	"setpgid": true, "getpgid": true, "getsid": true, "setsid": true,
	"sleep": true, "usleep": true, "nanosleep": true,
	"gethostname": true, "sethostname": true, "sysconf": true, "pathconf": true, "fpathconf": true,
	"access": true, "chdir": true, "fchdir": true, "getcwd": true, "getwd": true,
	"link": true, "symlink": true, "readlink": true, "unlink": true,
	"rmdir": true, "mkdir": true, "chmod": true, "fchmod": true, "chown": true, "fchown": true,
	"lchown": true, "truncate": true, "ftruncate": true,
	"alarm": true, "pause": true, "sync": true, "fsync": true, "fdatasync": true,
	"getrlimit": true, "setrlimit": true, "getrusage": true, "umask": true,
	"getpriority": true, "setpriority": true, "nice": true,
	"gethostid": true, "sethostid": true, "swab": true,
	// POSIX — fcntl
	"fcntl": true, "creat": true, "F_DUPFD": true, "F_GETFD": true, "F_SETFD": true,
	"F_GETFL": true, "F_SETFL": true, "F_GETLK": true, "F_SETLK": true, "F_SETLKW": true,
	"FD_CLOEXEC": true, "O_RDONLY": true, "O_WRONLY": true, "O_RDWR": true,
	"O_CREAT": true, "O_EXCL": true, "O_TRUNC": true, "O_APPEND": true, "O_NONBLOCK": true,
	"O_DSYNC": true, "O_SYNC": true, "O_RSYNC": true, "O_CLOEXEC": true,
	// POSIX — sys/stat
	"stat": true, "fstat": true, "lstat": true, "mkfifo": true, "mknod": true,
	"S_IFMT": true, "S_IFDIR": true, "S_IFREG": true, "S_IFCHR": true, "S_IFBLK": true,
	"S_IFIFO": true, "S_IFLNK": true, "S_IFSOCK": true,
	"S_IRWXU": true, "S_IRUSR": true, "S_IWUSR": true, "S_IXUSR": true,
	"S_IRWXG": true, "S_IRGRP": true, "S_IWGRP": true, "S_IXGRP": true,
	"S_IRWXO": true, "S_IROTH": true, "S_IWOTH": true, "S_IXOTH": true,
	"S_ISUID": true, "S_ISGID": true, "S_ISVTX": true,
	// POSIX — dirent
	"opendir": true, "readdir": true, "closedir": true, "rewinddir": true,
	"seekdir": true, "telldir": true, "DIR": true, "dirent": true, "scandir": true,
	"alphasort": true, "versionsort": true,
	// POSIX — sys/mman
	"mmap": true, "munmap": true, "mprotect": true, "msync": true,
	"mlock": true, "munlock": true, "mlockall": true, "munlockall": true,
	"shm_open": true, "shm_unlink": true, "shmget": true, "shmat": true, "shmdt": true, "shmctl": true,
	"sem_open": true, "sem_close": true, "sem_unlink": true, "sem_wait": true,
	"sem_trywait": true, "sem_timedwait": true, "sem_post": true, "sem_getvalue": true,
	"sem_init": true, "sem_destroy": true,
	"PROT_READ": true, "PROT_WRITE": true, "PROT_EXEC": true, "PROT_NONE": true,
	"MAP_SHARED": true, "MAP_PRIVATE": true, "MAP_ANONYMOUS": true, "MAP_ANON": true,
	"MAP_FAILED": true, "MS_SYNC": true, "MS_ASYNC": true, "MS_INVALIDATE": true,
	// POSIX — pthread
	"pthread_create": true, "pthread_join": true, "pthread_detach": true,
	"pthread_exit": true, "pthread_self": true, "pthread_equal": true,
	"pthread_cancel": true, "pthread_testcancel": true, "pthread_setcanceltype": true,
	"pthread_mutex_init": true, "pthread_mutex_lock": true, "pthread_mutex_unlock": true,
	"pthread_mutex_destroy": true, "pthread_mutex_trylock": true, "pthread_mutex_timedlock": true,
	"pthread_cond_init": true, "pthread_cond_wait": true, "pthread_cond_signal": true,
	"pthread_cond_broadcast": true, "pthread_cond_destroy": true, "pthread_cond_timedwait": true,
	"pthread_rwlock_init": true, "pthread_rwlock_rdlock": true, "pthread_rwlock_wrlock": true,
	"pthread_rwlock_unlock": true, "pthread_rwlock_destroy": true, "pthread_rwlock_tryrdlock": true,
	"pthread_rwlock_trywrlock": true, "pthread_rwlock_timedrdlock": true, "pthread_rwlock_timedwrlock": true,
	"pthread_once": true, "pthread_key_create": true, "pthread_key_delete": true,
	"pthread_setspecific": true, "pthread_getspecific": true,
	"pthread_attr_init": true, "pthread_attr_destroy": true, "pthread_attr_setdetachstate": true,
	"pthread_attr_getdetachstate": true, "pthread_attr_setstacksize": true, "pthread_attr_getstacksize": true,
	"pthread_attr_setstack": true, "pthread_attr_getstack": true,
	"PTHREAD_CREATE_JOINABLE": true, "PTHREAD_CREATE_DETACHED": true,
	"PTHREAD_MUTEX_INITIALIZER": true, "PTHREAD_COND_INITIALIZER": true,
	"PTHREAD_ONCE_INIT": true, "PTHREAD_RWLOCK_INITIALIZER": true,
	// POSIX — signal 补充
	"sigaction": true, "sigprocmask": true, "sigpending": true, "sigsuspend": true,
	"sigemptyset": true, "sigfillset": true, "sigaddset": true, "sigdelset": true, "sigismember": true,
	"kill": true, "sigqueue": true, "SIGHUP": true, "SIGQUIT": true, "SIGUSR1": true, "SIGUSR2": true,
	"SIGPIPE": true, "SIGALRM": true, "SIGCHLD": true, "SIGCONT": true, "SIGSTOP": true,
	"SIGTSTP": true, "SIGTTIN": true, "SIGTTOU": true, "SIGBUS": true, "SIGPOLL": true,
	"SIGPROF": true, "SIGSYS": true, "SIGTRAP": true, "SIGURG": true, "SIGVTALRM": true,
	"SIGXCPU": true, "SIGXFSZ": true, "SA_SIGINFO": true, "SA_RESTART": true,
	// POSIX — 套接字 / sys/socket (补充)
	"socket": true, "listen": true, "accept": true, "connect": true,
	"send": true, "recv": true, "sendto": true, "recvfrom": true, "sendmsg": true, "recvmsg": true,
	"setsockopt": true, "getsockopt": true, "getsockname": true, "getpeername": true,
	"shutdown": true, "sockaddr": true, "sockaddr_in": true, "sockaddr_in6": true, "sockaddr_un": true,
	"AF_UNIX": true, "AF_INET": true, "AF_INET6": true,
	"SOCK_STREAM": true, "SOCK_DGRAM": true, "SOCK_RAW": true, "SOCK_SEQPACKET": true,
	"SOL_SOCKET": true, "SO_REUSEADDR": true, "SO_REUSEPORT": true, "SO_KEEPALIVE": true,
	"SO_LINGER": true, "SO_BROADCAST": true, "SO_OOBINLINE": true, "SO_SNDBUF": true, "SO_RCVBUF": true,
	"SO_SNDTIMEO": true, "SO_RCVTIMEO": true, "SO_ERROR": true, "SO_TYPE": true,
	"IPPROTO_TCP": true, "IPPROTO_UDP": true, "IPPROTO_IP": true, "IPPROTO_IPV6": true,
	"TCP_NODELAY": true, "MSG_OOB": true, "MSG_PEEK": true, "MSG_DONTROUTE": true,
	"MSG_WAITALL": true, "SHUT_RD": true, "SHUT_WR": true, "SHUT_RDWR": true,
	"hostent": true, "gethostbyname": true, "gethostbyaddr": true, "getservbyname": true,
	"getprotobyname": true, "htons": true, "htonl": true, "ntohs": true, "ntohl": true,
	"inet_addr": true, "inet_ntoa": true, "inet_pton": true, "inet_ntop": true,
	"addrinfo": true, "getaddrinfo": true, "freeaddrinfo": true, "gai_strerror": true,
	"poll": true, "ppoll": true, "select": true, "pselect": true, "epoll_create": true,
	"epoll_ctl": true, "epoll_wait": true, "EPOLLIN": true, "EPOLLOUT": true, "EPOLLERR": true,
	"EPOLLET": true, "EPOLLONESHOT": true,
	// 常用宏和常量
	"EOF": true, "BUFSIZ": true, "SEEK_SET": true, "SEEK_CUR": true, "SEEK_END": true,
	"EXIT_SUCCESS": true, "EXIT_FAILURE": true, "NULL": true, "stdin": true, "stdout": true,
	"stderr": true, "TRUE": true, "FALSE": true, "STDIN_FILENO": true,
	"STDOUT_FILENO": true, "STDERR_FILENO": true, "F_OK": true, "R_OK": true,
	"W_OK": true, "X_OK": true,
	"PATH_MAX": true, "NAME_MAX": true, "ARG_MAX": true, "PIPE_BUF": true,
	// C++ 异常 / exception
	"exception": true, "bad_exception": true, "bad_alloc": true, "bad_array_new_length": true,
	"logic_error": true, "domain_error": true, "invalid_argument": true, "length_error": true,
	"out_of_range": true, "runtime_error": true, "range_error": true, "overflow_error": true,
	"underflow_error": true, "regex_error": true, "system_error": true, "ios_base::failure": true,
	"filesystem::filesystem_error": true, "future_error": true,
	"current_exception": true, "rethrow_exception": true, "make_exception_ptr": true,
	"throw_with_nested": true, "rethrow_if_nested": true, "nested_exception": true,
	"terminate": true, "set_terminate": true, "unexpected": true, "set_unexpected": true,
	// C++ typeinfo
	"type_index": true,
	// C 常见 FILE 类型
	"FILE": true, "fpos_t": true, "va_list": true,
}

func isCPPKeyword(name string) bool {
	return cppKeywords[name]
}

func filterCPPKeywords(deps []string) []string {
	filtered := make([]string, 0, len(deps))
	for _, d := range deps {
		if !isCPPKeyword(d) {
			filtered = append(filtered, d)
		}
	}
	return filtered
}

// makePreprocessorMask 标记 C/C++ 预处理指令行和死代码块
// 1. #if 0 ... #endif — 条件编译死代码
// 2. #define 宏定义及其续行体 — 宏展开不在语义层面可见
// 3. 其他 # 指令行直接跳过（由调用方处理）
func makePreprocessorMask(lines []string) []bool {
	mask := make([]bool, len(lines))
	inIfZero := false    // 在 #if 0 ... #endif 块内
	inDefine := false    // 在 #define 续行内
	ifZeroNest := 0      // 嵌套 #if/#ifdef/#ifndef 计数

	for i, line := range lines {
		trimmed := strings.TrimSpace(line)

		// 处理多行宏定义续行
		if inDefine {
			mask[i] = true
			// 检查续行是否结束（行尾没有 \）
			if !strings.HasSuffix(strings.TrimRight(line, " \t\r"), "\\") {
				inDefine = false
			}
			continue
		}

		// 处理 #if 0 ... #endif 块
		if inIfZero {
			mask[i] = true
			if strings.HasPrefix(trimmed, "#if") || strings.HasPrefix(trimmed, "#ifdef") || strings.HasPrefix(trimmed, "#ifndef") {
				ifZeroNest++
			} else if strings.HasPrefix(trimmed, "#endif") {
				if ifZeroNest > 0 {
					ifZeroNest--
				} else {
					inIfZero = false
				}
			}
			continue
		}

		// 检测 # 预处理指令
		if strings.HasPrefix(trimmed, "#") {
			if trimmed == "#if 0" {
				// #if 0 — 死代码块开始
				inIfZero = true
				mask[i] = true
				ifZeroNest = 0
			} else if strings.HasPrefix(trimmed, "#if ") && strings.Contains(trimmed, "0") {
				// 可能还有其他 #if 0 变体
				parts := strings.Fields(trimmed)
				for _, p := range parts {
					if p == "0" {
						inIfZero = true
						mask[i] = true
						ifZeroNest = 0
						break
					}
				}
				mask[i] = true
			} else if strings.HasPrefix(trimmed, "#define") {
				// #define — 标记此行及所有续行
				mask[i] = true
				if strings.HasSuffix(strings.TrimRight(line, " \t\r"), "\\") {
					inDefine = true
				}
			} else {
				// 其他 # 指令（#include, #ifdef, #ifndef, #else, #elif, #pragma 等）
				mask[i] = true
			}
			continue
		}
	}

	return mask
}
