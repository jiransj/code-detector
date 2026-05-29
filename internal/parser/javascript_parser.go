package parser

import (
	"regexp"
	"strings"

	"code-detector/internal/model"
)

// JSParser - parse  JavaScript/TypeScript 
type JSParser struct {
	language string // "javascript"  "typescript"
}

func NewJavascriptParser() *JSParser { return &JSParser{language: "javascript"} }
func NewTypescriptParser() *JSParser { return &JSParser{language: "typescript"} }

func (p *JSParser) Language() string { return p.language }

// extractJSModuleName  JS extract 
//  package.json  import/export 
func extractJSModuleName(filePath string, content []byte) string {
	text := string(content)
	lines := strings.Split(text, "\n")
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "export default class ") {
			parts := strings.Fields(trimmed)
			if len(parts) >= 4 {
				return parts[3]
			}
		}
		if strings.HasPrefix(trimmed, "export default function ") {
			parts := strings.Fields(trimmed)
			if len(parts) >= 4 {
				return parts[3]
			}
		}
	}
	return extractModuleNameFromPath(filePath)
}

// extractModuleNameFromPath 
func extractModuleNameFromPath(filePath string) string {
	parts := strings.Split(strings.ReplaceAll(filePath, "\\", "/"), "/")
	if len(parts) >= 2 {
		return parts[len(parts)-2]
	}
	return "main"
}

// Parse -- parse  JS/TS 
func (p *JSParser) Parse(filePath string, content []byte) ([]*model.Function, error) {
	fl := NewFileLinesFromBytes(content)
	lines := fl.Lines()
	text := fl.Text()

	commentMask := makeJSCommentMask(lines)
	stringMask := makeJSStringMask(lines)

	// function name() / name = function() / name = () => / async function name()
	funcDefRegex := regexp.MustCompile(`(?:async\s+)?function\s+(?P<name>\w+)\s*\(|\b(\w+)\s*[:=]\s*(?:async\s+)?function\s*\(|\b(\w+)\s*[:=]\s*\([^)]*\)\s*=>`)

	type jsFuncStart struct {
		lineIdx int
		name    string
	}
	var starts []jsFuncStart

	for i, line := range lines {
		if commentMask[i] || stringMask[i] {
			continue
		}
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.HasPrefix(trimmed, "//") {
			continue
		}

		matches := funcDefRegex.FindStringSubmatch(line)
		if matches == nil {
			continue
		}

		name := ""
		for _, m := range matches[1:] {
			if m != "" {
				name = m
				break
			}
		}
		if name == "" || name == "if" || name == "else" || name == "for" || name == "while" || name == "switch" || name == "catch" || name == "then" {
			continue
		}

		starts = append(starts, jsFuncStart{lineIdx: i, name: name})
	}

	if len(starts) == 0 {
		return nil, nil
	}

	moduleName := extractJSModuleName(filePath, content)
	allFuncs := make([]*model.Function, 0, len(starts))

	for _, fs := range starts {
		offset := fl.LineOffset(fs.lineIdx)
		line := lines[fs.lineIdx]
		braceIdx := findBraceInLine(line)
		if braceIdx < 0 {
			for j := fs.lineIdx + 1; j < len(lines); j++ {
				if commentMask[j] || stringMask[j] {
					continue
				}
				if idx := findBraceInLine(lines[j]); idx >= 0 {
					braceIdx = idx
					offset = fl.LineOffset(j) + idx
					break
				}
			}
			if braceIdx < 0 {
				continue
			}
		} else {
			offset += braceIdx
		}

		closeOffset, err := matchBrace(text, offset)
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

		callStats := extractCallStatsSimple(body, jsCallRegex, isJSStdFunction)

		f := &model.Function{
			Name:         fs.name,
			PackageName:  moduleName,
			Language:     p.language,
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

// Globals extract  JS/TS 
func (p *JSParser) Globals(filePath string, content []byte) ([]*model.GlobalVariable, error) {
	text := string(content)
	lines := strings.Split(text, "\n")
	commentMask := makeJSCommentMask(lines)
	stringMask := makeJSStringMask(lines)

	var vars []*model.GlobalVariable

	//  const/let/var 
	varDeclRegex := regexp.MustCompile(`^(?:export\s+)?(?:const|let|var)\s+(?P<name>\w+)\s*(?:=|;)`)

	for i, line := range lines {
		if commentMask[i] || stringMask[i] {
			continue
		}
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.HasPrefix(trimmed, "//") || strings.HasPrefix(trimmed, "/*") {
			continue
		}

		matches := varDeclRegex.FindStringSubmatch(line)
		if matches == nil {
			continue
		}
		nameIdx := varDeclRegex.SubexpIndex("name")
		if nameIdx < 0 || nameIdx >= len(matches) {
			continue
		}
		name := matches[nameIdx]
		if name == "" {
			continue
		}
		vars = append(vars, &model.GlobalVariable{
			Name:     name,
			Language: p.language,
			FilePath: filePath,
			LineNum:  i + 1,
			IsConst:  strings.Contains(trimmed, "const"),
		})
	}
	return vars, nil
}

// makeJSCommentMask  JS/TS 
func makeJSCommentMask(lines []string) []bool {
	mask := make([]bool, len(lines))
	inBlock := false
	inBlockEnd := -1

	for i, line := range lines {
		if inBlock {
			mask[i] = true
			if i <= inBlockEnd {
				continue
			}
			inBlock = false
		}

		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			continue
		}

		if strings.HasPrefix(trimmed, "//") {
			mask[i] = true
			continue
		}

		if strings.HasPrefix(trimmed, "/*") {
			mask[i] = true
			endIdx := strings.Index(trimmed, "*/")
			if endIdx < 0 {
				inBlock = true
				// 
				for j := i + 1; j < len(lines); j++ {
					if strings.Contains(lines[j], "*/") {
						inBlockEnd = j
						mask[j] = true
						break
					}
					mask[j] = true
				}
			}
			continue
		}

		if strings.Contains(trimmed, "//") {
			commentIdx := strings.Index(trimmed, "//")
			before := trimmed[:commentIdx]
			if !strings.Contains(before, "\"") && !strings.Contains(before, "'") && !strings.Contains(before, "`") {
				mask[i] = true
			}
		}
	}
	return mask
}

// makeJSStringMask  JS/TS 
func makeJSStringMask(lines []string) []bool {
	mask := make([]bool, len(lines))
	for i, line := range lines {
		inDouble := false
		inSingle := false
		inBacktick := false
		escaped := false

		for _, ch := range line {
			if escaped {
				escaped = false
				continue
			}
			if ch == '\\' && (inDouble || inSingle || inBacktick) {
				escaped = true
				continue
			}
			if ch == '"' && !inSingle && !inBacktick {
				inDouble = !inDouble
			} else if ch == '\'' && !inDouble && !inBacktick {
				inSingle = !inSingle
			} else if ch == '`' && !inDouble && !inSingle {
				inBacktick = !inBacktick
			}
		}
		mask[i] = inDouble || inSingle || inBacktick
	}
	return mask
}

// isJSStmtKeyword  JavaScript 
func isJSStmtKeyword(name string) bool {
	switch name {
	case "if", "else", "for", "while", "do", "switch", "case",
		"try", "catch", "finally", "with", "import", "export",
		"class", "function", "async", "await", "yield", "return",
		"throw", "new", "debugger":
		return true
	}
	return false
}

// isJSKeyword  JS 
func isJSKeyword(name string) bool {
	switch name {
	case "if", "else", "for", "while", "do",
		"switch", "case", "default", "break",
		"continue", "return", "throw", "try",
		"catch", "finally", "new", "this",
		"super", "typeof", "instanceof", "void",
		"delete", "in", "of", "yield", "await",
		"async", "function", "class", "extends",
		"import", "export", "from", "const",
		"let", "var", "null", "undefined",
		"true", "false", "NaN", "Infinity":
		return true
	}
	return false
}

func filterJSKeywords(deps []string) []string {
	filtered := make([]string, 0, len(deps))
	for _, d := range deps {
		if !isJSKeyword(d) {
			filtered = append(filtered, d)
		}
	}
	return filtered
}

// jsCallRegex  JavaScript  qualified call 
var jsCallRegex = regexp.MustCompile(`(?:(\w+)\.)?(\w+)\s*\(`)

// jsStdFunctions  JavaScript/TypeScript /API map 
var jsStdFunctions = map[string]bool{
	"if": true, "else": true, "for": true, "while": true, "do": true,
	"switch": true, "case": true, "default": true, "break": true,
	"continue": true, "return": true, "throw": true, "try": true,
	"catch": true, "finally": true, "new": true, "this": true,
	"super": true, "typeof": true, "instanceof": true, "void": true,
	"delete": true, "in": true, "of": true, "yield": true, "await": true,
	"async": true, "function": true, "class": true, "extends": true,
	"import": true, "export": true, "from": true, "const": true,
	"let": true, "var": true, "null": true, "undefined": true,
	"true": true, "false": true, "NaN": true, "Infinity": true,
	// 全局对象 / CommonJS
	"console": true, "require": true, "define": true, "module": true, "exports": true,
	"global": true, "globalThis": true, "__dirname": true, "__filename": true,
	// 内置类型
	"String": true, "Number": true, "Boolean": true, "Array": true,
	"Object": true, "Function": true, "Date": true, "RegExp": true,
	"Map": true, "Set": true, "Promise": true, "Symbol": true, "WeakMap": true, "WeakSet": true,
	"Int8Array": true, "Uint8Array": true, "Uint8ClampedArray": true, "Int16Array": true,
	"Uint16Array": true, "Int32Array": true, "Uint32Array": true, "Float32Array": true, "Float64Array": true,
	"BigInt64Array": true, "BigUint64Array": true, "ArrayBuffer": true, "SharedArrayBuffer": true,
	"DataView": true, "BigInt": true,
	"parseInt": true, "parseFloat": true, "isNaN": true, "isFinite": true,
	"JSON": true, "Math": true, "Reflect": true, "Proxy": true,
	"Error": true, "TypeError": true, "RangeError": true, "ReferenceError": true,
	"SyntaxError": true, "EvalError": true, "URIError": true, "AggregateError": true,
	"setTimeout": true, "setInterval": true, "clearTimeout": true,
	"clearInterval": true, "setImmediate": true, "clearImmediate": true,
	"queueMicrotask": true, "requestAnimationFrame": true, "cancelAnimationFrame": true,
	"structuredClone": true,
	// Array 方法
	"map": true, "filter": true, "reduce": true, "reduceRight": true, "forEach": true,
	"find": true, "findIndex": true, "findLast": true, "findLastIndex": true,
	"some": true, "every": true, "includes": true, "copyWithin": true,
	"push": true, "pop": true, "shift": true, "unshift": true, "slice": true, "splice": true,
	"concat": true, "join": true, "indexOf": true, "lastIndexOf": true, "sort": true,
	"reverse": true, "fill": true, "flat": true, "flatMap": true, "at": true, "toReversed": true,
	"toSorted": true, "toSpliced": true, "with": true, "group": true, "groupToMap": true,
	// String 方法
	"charAt": true, "charCodeAt": true, "codePointAt": true, "match": true, "matchAll": true,
	"replace": true, "replaceAll": true, "search": true,
	"split": true, "substring": true, "substr": true,
	"toLowerCase": true, "toUpperCase": true, "localeCompare": true, "normalize": true,
	"trim": true, "trimStart": true, "trimEnd": true, "padStart": true, "padEnd": true,
	"startsWith": true, "endsWith": true, "repeat": true,
	// Object 方法
	"keys": true, "values": true, "entries": true, "assign": true, "create": true,
	"defineProperty": true, "defineProperties": true, "freeze": true, "fromEntries": true,
	"getOwnPropertyDescriptor": true, "getOwnPropertyDescriptors": true,
	"getOwnPropertyNames": true, "getOwnPropertySymbols": true,
	"getPrototypeOf": true, "is": true, "isFrozen": true, "isSealed": true, "isExtensible": true,
	"preventExtensions": true, "seal": true, "setPrototypeOf": true, "hasOwn": true,
	"groupBy": true,
	// Promise 方法
	"all": true, "race": true, "allSettled": true, "any": true, "resolve": true, "reject": true, "then": true,
	"withResolvers": true,
	// Math 方法
	"abs": true, "ceil": true, "floor": true, "round": true, "max": true, "min": true, "pow": true,
	"sqrt": true, "random": true, "trunc": true, "sign": true, "cbrt": true, "hypot": true, "clz32": true,
	"imul": true, "fround": true, "expm1": true, "log1p": true, "log2": true, "log10": true,
	"sin": true, "cos": true, "tan": true, "asin": true, "acos": true, "atan": true, "atan2": true,
	"sinh": true, "cosh": true, "tanh": true, "asinh": true, "acosh": true, "atanh": true,
	"exp": true, "log": true,
	// URI / 编码
	"fetch": true, "decodeURI": true, "decodeURIComponent": true,
	"encodeURI": true, "encodeURIComponent": true, "escape": true, "unescape": true,
	"atob": true, "btoa": true,
	// DOM / Web API
	"document": true, "querySelector": true, "querySelectorAll": true, "documentElement": true,
	"getElementById": true, "createElement": true, "addEventListener": true,
	"removeEventListener": true, "dispatchEvent": true,
	"localStorage": true, "sessionStorage": true,
	"setItem": true, "getItem": true, "removeItem": true, "key": true,
	"window": true, "navigator": true, "history": true, "location": true, "screen": true,
	"userAgent": true, "pushState": true, "replaceState": true, "go": true, "back": true, "forward": true,
	"innerWidth": true, "innerHeight": true, "outerWidth": true, "outerHeight": true,
	"scrollTo": true, "scrollBy": true, "scrollIntoView": true,
	"XMLHttpRequest": true, "FormData": true, "File": true, "FileReader": true,
	"Blob": true, "WebSocket": true, "MessageChannel": true,
	"IntersectionObserver": true, "MutationObserver": true, "ResizeObserver": true,
	"PerformanceObserver": true, "ReportingObserver": true,
	"AbortController": true, "AbortSignal": true,
	"Event": true, "CustomEvent": true, "EventTarget": true,
	"Node": true, "Element": true, "HTMLElement": true, "HTMLDivElement": true,
	"textContent": true, "innerHTML": true, "innerText": true, "classList": true,
	"appendChild": true, "removeChild": true, "replaceChild": true, "insertBefore": true,
	"closest": true, "matches": true, "contains": true, "hasAttribute": true,
	"getAttribute": true, "setAttribute": true, "removeAttribute": true,
	"style": true, "getComputedStyle": true,
	"Headers": true, "Request": true, "Response": true,
	"crypto": true, "subtle": true, "getRandomValues": true, "randomUUID": true,
	"Intl": true, "DateTimeFormat": true, "NumberFormat": true, "Collator": true,
	"ListFormat": true, "PluralRules": true, "RelativeTimeFormat": true,
	"Segmenter": true, "Segments": true,
	"Performance": true, "performance": true, "now": true, "mark": true, "measure": true,
	"warn": true, "table": true, "time": true, "timeEnd": true, "timeLog": true,
	"groupEnd": true, "groupCollapsed": true, "trace": true, "count": true, "countReset": true,
	"profile": true, "profileEnd": true,
	// Node.js — process
	"process": true, "cwd": true, "chdir": true, "env": true, "argv": true, "exit": true,
	"pid": true, "ppid": true, "platform": true, "arch": true, "title": true, "uptime": true,
	"hrtime": true, "memoryUsage": true, "cpuUsage": true,
	"nextTick": true, "stdin": true, "stdout": true, "stderr": true,
	"emit": true, "listeners": true,
	"removeAllListeners": true, "setMaxListeners": true, "getMaxListeners": true,
	"config": true, "features": true, "release": true, "version": true, "versions": true,
	// Node.js — Buffer
	"Buffer": true, "alloc": true, "allocUnsafe": true, "allocUnsafeSlow": true,
	"byteLength": true, "compare": true, "isBuffer": true,
	"isEncoding": true, "poolSize": true, "transcode": true,
	"readBigInt64BE": true, "readBigInt64LE": true, "readDoubleBE": true,
	"readDoubleLE": true, "readFloatBE": true, "readFloatLE": true, "readInt8": true,
	"readInt16BE": true, "readInt16LE": true, "readInt32BE": true, "readInt32LE": true,
	"readUInt8": true, "readUInt16BE": true, "readUInt16LE": true, "readUInt32BE": true,
	"readUInt32LE": true, "subarray": true, "swap16": true, "swap32": true,
	"swap64": true, "toJSON": true, "writeBigInt64BE": true,
	"writeBigInt64LE": true, "writeDoubleBE": true, "writeDoubleLE": true,
	"writeFloatBE": true, "writeFloatLE": true, "writeInt8": true, "writeInt16BE": true,
	"writeInt16LE": true, "writeInt32BE": true, "writeInt32LE": true, "writeUInt8": true,
	"writeUInt16BE": true, "writeUInt16LE": true, "writeUInt32BE": true, "writeUInt32LE": true,
	// Node.js — fs
	"access": true, "accessSync": true, "appendFile": true, "appendFileSync": true,
	"chmod": true, "chmodSync": true, "chown": true, "chownSync": true,
	"close": true, "closeSync": true, "copyFile": true, "copyFileSync": true,
	"cp": true, "cpSync": true, "createReadStream": true, "createWriteStream": true,
	"exists": true, "existsSync": true,
	"fchmod": true, "fchmodSync": true, "fchown": true, "fchownSync": true,
	"fdatasync": true, "fdatasyncSync": true, "fstat": true, "fstatSync": true,
	"fsync": true, "fsyncSync": true, "ftruncate": true, "ftruncateSync": true,
	"futimes": true, "futimesSync": true, "lchmod": true, "lchmodSync": true,
	"lchown": true, "lchownSync": true, "link": true, "linkSync": true,
	"lstat": true, "lstatSync": true, "mkdir": true, "mkdirSync": true,
	"mkdtemp": true, "mkdtempSync": true, "open": true, "openSync": true,
	"opendir": true, "opendirSync": true, "read": true, "readSync": true,
	"readdir": true, "readdirSync": true, "readFile": true, "readFileSync": true,
	"readlink": true, "readlinkSync": true, "readv": true, "readvSync": true,
	"realpath": true, "realpathSync": true, "rename": true, "renameSync": true,
	"rm": true, "rmSync": true, "rmdir": true, "rmdirSync": true,
	"stat": true, "statSync": true, "statfs": true, "statfsSync": true,
	"symlink": true, "symlinkSync": true, "truncate": true, "truncateSync": true,
	"unlink": true, "unlinkSync": true, "unwatchFile": true, "utimes": true, "utimesSync": true,
	"watch": true, "watchFile": true, "write": true, "writeSync": true,
	"writeFile": true, "writeFileSync": true, "writev": true, "writevSync": true,
	// Node.js — path
	"basename": true, "delimiter": true, "dirname": true, "extname": true,
	"isAbsolute": true, "relative": true, "sep": true, "toNamespacedPath": true,
	"win32": true, "posix": true,
	// Node.js — http / https
	"createServer": true, "request": true, "get": true, "Agent": true,
	"Server": true, "ServerResponse": true, "IncomingMessage": true,
	"globalAgent": true, "METHODS": true, "STATUS_CODES": true,
	"createConnection": true, "connect": true, "maxHeaderSize": true,
	// Node.js — os
	"cpus": true, "endianness": true, "freemem": true,
	"getPriority": true, "homedir": true, "hostname": true, "loadavg": true,
	"networkInterfaces": true,
	"setPriority": true, "tmpdir": true, "totalmem": true,
	"userInfo": true, "machine": true, "devNull": true, "EOL": true,
	// Node.js — crypto
	"createHash": true, "createHmac": true, "createCipheriv": true, "createDecipheriv": true,
	"createSign": true, "createVerify": true, "createDiffieHellman": true,
	"createECDH": true, "pbkdf2": true, "pbkdf2Sync": true, "randomBytes": true,
	"randomFill": true, "randomFillSync": true, "randomInt": true, "scrypt": true,
	"scryptSync": true, "timingSafeEqual": true, "generateKeyPair": true,
	"generateKeyPairSync": true, "generateKey": true, "generateKeySync": true,
	"Hash": true, "Hmac": true, "Cipher": true, "Decipher": true, "Sign": true, "Verify": true,
	"DiffieHellman": true, "DiffieHellmanGroup": true, "ECDH": true,
	"getCiphers": true, "getCurves": true, "getHashes": true,
	"privateDecrypt": true, "privateEncrypt": true, "publicDecrypt": true, "publicEncrypt": true,
	"certificate": true, "Certificate": true, "checkPrime": true, "checkPrimeSync": true,
	"createSecretKey": true, "createPublicKey": true, "createPrivateKey": true,
	"KeyObject": true, "X509Certificate": true,
	// Node.js — stream
	"Readable": true, "Writable": true, "Transform": true, "Duplex": true, "PassThrough": true,
	"pipeline": true, "pipelineSync": true, "finished": true, "promises": true,
	"addAbortSignal": true, "isReadable": true, "isWritable": true, "isErrored": true,
	"isDisturbed": true, "destroy": true, "setEncoding": true, "pause": true, "resume": true,
	// Node.js — events
	"EventEmitter": true, "once": true, "captureRejections": true,
	"defaultMaxListeners": true, "errorMonitor": true,
	// Node.js — util
	"promisify": true, "callbackify": true, "inherits": true, "format": true, "formatWithOptions": true,
	"deprecate": true, "isArray": true, "isBoolean": true,
	"isDate": true, "isError": true, "isFunction": true, "isNull": true, "isNullOrUndefined": true,
	"isNumber": true, "isObject": true, "isPrimitive": true, "isRegExp": true,
	"isString": true, "isSymbol": true, "isUndefined": true,
	"debuglog": true, "extend": true, "getSystemErrorMap": true, "getSystemErrorName": true,
	"stripVTControlCharacters": true, "toUSVString": true, "TextDecoder": true, "TextEncoder": true,
	"types": true, "MIMEType": true, "MIMEParams": true, "parseArgs": true,
	// Node.js — child_process
	"exec": true, "execSync": true, "execFile": true, "execFileSync": true,
	"fork": true, "spawn": true, "spawnSync": true, "ChildProcess": true,
	// Node.js — url
	"URL": true, "URLSearchParams": true,
	"domainToASCII": true, "domainToUnicode": true, "pathToFileURL": true, "fileURLToPath": true,
	// Node.js — querystring
	"stringify": true,
	// Node.js — assert
	"strict": true, "ok": true, "deepEqual": true, "deepStrictEqual": true,
	"doesNotReject": true, "doesNotThrow": true, "equal": true, "fail": true,
	"ifError": true, "not": true, "notDeepEqual": true, "notDeepStrictEqual": true,
	"notEqual": true, "notStrictEqual": true, "rejects": true, "strictEqual": true,
	"throws": true, "CallTracker": true, "Snapshot": true, "AssertionError": true,
	// Node.js — net
	"Socket": true, "isIP": true, "isIPv4": true, "isIPv6": true, "BlockList": true,
	"getDefaultAutoSelectFamily": true, "setDefaultAutoSelectFamily": true,
	// Node.js — dns
	"lookup": true, "lookupService": true, "resolve4": true, "resolve6": true,
	"resolveAny": true, "resolveCaa": true, "resolveCname": true, "resolveMx": true,
	"resolveNaptr": true, "resolveNs": true, "resolvePtr": true, "resolveSoa": true,
	"resolveSrv": true, "resolveTxt": true, "setServers": true,
	"getServers": true,
	// Node.js — readline
	"createInterface": true, "Interface": true, "emitKeypressEvents": true,
	"clearLine": true, "clearScreenDown": true, "cursorTo": true, "moveCursor": true,
	// Node.js — zlib
	"brotliCompress": true, "brotliCompressSync": true, "brotliDecompress": true, "brotliDecompressSync": true,
	"deflate": true, "deflateSync": true, "deflateRaw": true, "deflateRawSync": true,
	"gunzip": true, "gunzipSync": true, "gzip": true, "gzipSync": true,
	"inflate": true, "inflateSync": true, "inflateRaw": true, "inflateRawSync": true,
	"unzip": true, "unzipSync": true, "constants": true, "createDeflate": true,
	"createDeflateRaw": true, "createGunzip": true, "createGzip": true, "createInflate": true,
	"createInflateRaw": true, "createUnzip": true, "createBrotliCompress": true, "createBrotliDecompress": true,
	// Node.js — cluster
	"disconnect": true, "isMaster": true, "isPrimary": true, "isWorker": true,
	"schedulingPolicy": true, "settings": true, "setupPrimary": true, "setupMaster": true,
	"workers": true,
	// TypeScript 工具类型
	"Partial": true, "Required": true, "Readonly": true, "Pick": true, "Omit": true,
	"Record": true, "Exclude": true, "Extract": true, "NonNullable": true,
	"ReturnType": true, "InstanceType": true, "Parameters": true, "ConstructorParameters": true,
	"Awaited": true, "ThisType": true, "ThisParameterType": true, "OmitThisParameter": true,
	"Uppercase": true, "Lowercase": true, "Capitalize": true, "Uncapitalize": true,
	"declare": true, "abstract": true, "readonly": true, "implements": true,
	"interface": true, "type": true, "satisfies": true,
}

func isJSStdFunction(name string) bool {
	return jsStdFunctions[name]
}
