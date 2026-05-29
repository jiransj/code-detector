package parser

import (
	"regexp"
	"strings"

	"code-detector/internal/model"
)

// CSharpParser 解析 C# 源文件
type CSharpParser struct{}

func NewCSharpParser() *CSharpParser { return &CSharpParser{} }
func (p *CSharpParser) Language() string { return "csharp" }

func (p *CSharpParser) Globals(filePath string, content []byte) ([]*model.GlobalVariable, error) {
	text := string(content)
	lines := strings.Split(text, "\n")
	commentMask := makeCommentMask(lines, []string{"//", "///"}, [][2]string{{"/*", "*/"}})

	var vars []*model.GlobalVariable
	csStaticRegex := regexp.MustCompile(`(?:public|private|protected|internal|static|readonly|const|volatile)\s+(?:static\s+)?(?:(?P<type>\w+(?:\<[^\>]*\>)?(?:\[\])*))\s+(?P<name>\w+)\s*(?:=|;)`)

	for i, line := range lines {
		if commentMask[i] {
			continue
		}
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.HasPrefix(trimmed, "//") || strings.HasPrefix(trimmed, "[") {
			continue
		}
		matches := csStaticRegex.FindStringSubmatch(line)
		if matches == nil {
			continue
		}
		nameIdx := csStaticRegex.SubexpIndex("name")
		typeIdx := csStaticRegex.SubexpIndex("type")
		name := matches[nameIdx]
		varType := ""
		if typeIdx >= 0 && typeIdx < len(matches) {
			varType = matches[typeIdx]
		}
		if name != "" {
			vars = append(vars, &model.GlobalVariable{
				Name: name, VarType: varType, Language: "csharp",
				FilePath: filePath, LineNum: i + 1,
				IsConst: strings.Contains(trimmed, "const "),
			})
		}
	}
	return vars, nil
}

var csFuncRegex = regexp.MustCompile(
	`(?:(?:public|private|protected|internal|static|virtual|override|abstract|async|unsafe|sealed|readonly|partial|new|extern)\s+)*(?:\w+(?:\[\])*(?:\<[^\>]+\>)?)\s+(?P<name>\w+)\s*\(`,
)
var csCallRegex = regexp.MustCompile(`(?:(\w+)\.)?(\w+)\s*\(`)

func (p *CSharpParser) Parse(filePath string, content []byte) ([]*model.Function, error) {
	fl := NewFileLinesFromBytes(content)
	lines := fl.Lines()
	text := fl.Text()

	commentMask := makeCommentMask(lines, []string{"//", "///"}, [][2]string{{"/*", "*/"}})
	stringMask := makeStringMask(lines)

	type csFuncStart struct {
		lineIdx int
		name    string
	}
	var starts []csFuncStart

	for i, line := range lines {
		if commentMask[i] || stringMask[i] {
			continue
		}
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.HasPrefix(trimmed, "#") || strings.HasPrefix(trimmed, "[") {
			continue
		}

		matches := csFuncRegex.FindStringSubmatch(line)
		if matches == nil {
			continue
		}
		nameIdx := csFuncRegex.SubexpIndex("name")
		if nameIdx < 0 || nameIdx >= len(matches) {
			continue
		}
		name := matches[nameIdx]
		if name == "" {
			continue
		}

		starts = append(starts, csFuncStart{lineIdx: i, name: name})
	}

	if len(starts) == 0 {
		return nil, nil
	}

	allFuncs := make([]*model.Function, 0, len(starts))
	for _, fs := range starts {
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

		callStats := extractCallStats(body, csCallRegex, stringMask, commentMask, startLine, endLine, isCSKeyword, nil)

		f := &model.Function{
			Name:         fs.name,
			Language:     "csharp",
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

// csKeywords 是 C# 关键字和常用标准库名的集合（map 数据驱动）
var csKeywords = map[string]bool{
	"if": true, "else": true, "for": true, "foreach": true, "while": true, "do": true, "switch": true, "case": true,
	"default": true, "break": true, "continue": true, "return": true, "throw": true, "try": true, "catch": true, "finally": true,
	"new": true, "this": true, "base": true, "as": true, "is": true, "in": true, "out": true, "ref": true, "sizeof": true,
	"typeof": true, "nameof": true, "checked": true, "unchecked": true, "unsafe": true, "fixed": true, "stackalloc": true,
	"await": true, "async": true, "yield": true, "from": true, "select": true, "where": true, "join": true, "group": true,
	"orderby": true, "let": true, "ascending": true, "descending": true, "equals": true, "by": true, "on": true, "into": true,
	"int": true, "long": true, "double": true, "float": true, "bool": true, "char": true, "byte": true, "short": true,
	"uint": true, "ulong": true, "ushort": true, "sbyte": true, "decimal": true, "string": true, "object": true, "void": true,
	"null": true, "true": true, "false": true, "var": true, "dynamic": true, "class": true, "struct": true, "enum": true,
	"interface": true, "record": true, "namespace": true, "using": true, "partial": true, "sealed": true, "static": true,
	"virtual": true, "override": true, "abstract": true, "public": true, "private": true, "protected": true, "internal": true,
	"readonly": true, "volatile": true, "const": true, "event": true, "delegate": true, "add": true, "remove": true,
	"set": true, "get": true, "value": true, "params": true, "implicit": true, "explicit": true, "operator": true,
	"global": true, "file": true, "required": true, "init": true, "with": true, "notnull": true, "managed": true, "unmanaged": true,
	// System 基础类型和工具
	"Console": true, "WriteLine": true, "Write": true, "ReadLine": true, "Read": true, "ReadKey": true,
	"Convert": true, "Math": true, "MathF": true, "BigInteger": true, "Complex": true,
	"String": true, "Task": true, "ValueTask": true, "List": true, "Dictionary": true, "IEnumerable": true,
	"IEnumerator": true, "IQueryable": true, "ICollection": true, "IList": true, "IDictionary": true,
	"Array": true, "Buffer": true, "Byte": true, "Char": true, "DateTime": true, "DateTimeOffset": true,
	"DateOnly": true, "TimeOnly": true, "Decimal": true, "Half": true,
	"Double": true, "Enum": true, "Environment": true, "Exception": true, "Guid": true, "HashCode": true,
	"Int16": true, "Int32": true, "Int64": true, "IntPtr": true, "UIntPtr": true, "nint": true, "nuint": true,
	"Lazy": true, "Nullable": true, "Object": true, "Random": true, "Stream": true, "StringBuilder": true,
	"TextReader": true, "TextWriter": true, "TimeSpan": true, "Tuple": true, "ValueTuple": true,
	"Type": true, "Uri": true, "Version": true, "WeakReference": true,
	"OperatingSystem": true, "PlatformID": true, "GC": true, "AppDomain": true, "Activator": true,
	"BitConverter": true, "Net": true,
	// System.Collections.Generic
	"HashSet": true, "Stack": true, "Queue": true, "LinkedList": true,
	"SortedList": true, "SortedDictionary": true, "SortedSet": true,
	"IComparer": true, "IEqualityComparer": true,
	// System.Collections.Immutable
	"ImmutableArray": true, "ImmutableList": true, "ImmutableDictionary": true,
	"ImmutableHashSet": true, "ImmutableStack": true, "ImmutableQueue": true, "ImmutableSortedSet": true, "ImmutableSortedDictionary": true,
	// System.IO
	"DriveInfo": true, "BufferedStream": true, "BinaryReader": true, "BinaryWriter": true,
	"StringReader": true, "StringWriter": true,
	"FileSystemWatcher": true, "FileSystemInfo": true, "FileAttributes": true, "SearchOption": true,
	"SeekOrigin": true,
	"Open": true, "OpenRead": true, "OpenWrite": true, "Create": true, "CreateText": true,
	"ReadAllText": true, "ReadAllLines": true, "ReadAllBytes": true,
	"WriteAllText": true, "WriteAllLines": true, "WriteAllBytes": true,
	"AppendAllText": true, "AppendAllLines": true, "Copy": true, "Delete": true,
	"Exists": true, "Move": true, "GetFiles": true, "GetDirectories": true,
	// System.Linq
	"Select": true, "Where": true, "OrderBy": true, "OrderByDescending": true,
	"ThenBy": true, "ThenByDescending": true, "GroupBy": true, "Join": true, "GroupJoin": true,
	"Skip": true, "Take": true, "SkipWhile": true, "TakeWhile": true, "SkipLast": true, "TakeLast": true,
	"First": true, "FirstOrDefault": true, "Last": true, "LastOrDefault": true,
	"Single": true, "SingleOrDefault": true, "ElementAt": true, "ElementAtOrDefault": true,
	"Any": true, "All": true, "Count": true, "LongCount": true, "Contains": true,
	"Min": true, "Max": true, "Sum": true, "Average": true, "Aggregate": true,
	"Distinct": true, "DistinctBy": true, "Union": true, "UnionBy": true,
	"Intersect": true, "IntersectBy": true, "Except": true, "ExceptBy": true,
	"ToList": true, "ToArray": true, "ToDictionary": true, "ToLookup": true, "ToHashSet": true,
	"Cast": true, "OfType": true, "AsEnumerable": true, "AsQueryable": true,
	"Range": true, "Repeat": true, "Empty": true, "DefaultIfEmpty": true,
	"SelectMany": true, "Zip": true, "Append": true, "Prepend": true,
	"Reverse": true, "Concat": true, "SequenceEqual": true,
	"Chunk": true, "MaxBy": true, "MinBy": true, "Order": true, "OrderDescending": true,
	// System.Threading
	"Thread": true, "ThreadPool": true, "Monitor": true, "Mutex": true, "Semaphore": true, "SemaphoreSlim": true,
	"Interlocked": true, "Volatile": true, "SpinLock": true, "SpinWait": true,
	"Barrier": true, "CountdownEvent": true, "ManualResetEvent": true, "AutoResetEvent": true,
	"ManualResetEventSlim": true, "ReaderWriterLock": true, "ReaderWriterLockSlim": true,
	"Timer": true, "CancellationToken": true, "CancellationTokenSource": true,
	"LockRecursionPolicy": true,
	// System.Threading.Tasks
	"TaskCompletionSource": true,
	"Parallel": true, "ParallelLoopResult": true, "ParallelOptions": true,
	"Run": true, "Start": true, "Wait": true, "Result": true, "ConfigureAwait": true,
	"Delay": true, "WhenAll": true, "WhenAny": true, "FromResult": true, "FromException": true, "FromCanceled": true,
	"Yield": true, "Sleep": true, "TaskStatus": true, "Unwrap": true,
	// System.Net
	"HttpClient": true, "HttpMessageHandler": true, "HttpRequestMessage": true, "HttpResponseMessage": true,
	"HttpMethod": true, "HttpStatusCode": true, "HttpContent": true, "StringContent": true,
	"WebClient": true, "WebRequest": true, "WebResponse": true, "HttpWebRequest": true, "HttpWebResponse": true,
	"IPAddress": true, "IPEndPoint": true, "Dns": true, "DnsEndPoint": true,
	"Socket": true, "TcpClient": true, "TcpListener": true, "UdpClient": true,
	"NetworkStream": true, "SslStream": true, "AuthenticateAsClient": true, "AuthenticateAsServer": true,
	"ServicePoint": true, "ServicePointManager": true, "Cookie": true, "CookieContainer": true,
	// System.Text
	"Encoding": true, "ASCII": true, "UTF8": true, "Unicode": true, "UTF32": true, "Latin1": true,
	"Encoder": true, "Decoder": true, "Rune": true,
	// System.Text.RegularExpressions
	"Regex": true, "Match": true, "MatchCollection": true, "Group": true, "Capture": true,
	"CaptureCollection": true, "GroupCollection": true, "RegexOptions": true,
	"IsMatch": true, "Matches": true, "Replace": true, "Split": true,
	"RegexCompilationInfo": true, "RegexRunner": true, "RegexRunnerFactory": true,
	// System.Text.Json
	"JsonSerializer": true, "JsonDocument": true, "JsonElement": true, "JsonNode": true,
	"JsonObject": true, "JsonArray": true, "JsonValue": true, "JsonProperty": true,
	"JsonSerializerOptions": true, "JsonWriterOptions": true, "JsonReaderOptions": true,
	"Utf8JsonWriter": true, "Utf8JsonReader": true,
	// System.Xml / System.Xml.Linq
	"XmlDocument": true, "XmlElement": true, "XmlNode": true, "XmlAttribute": true, "XmlReader": true, "XmlWriter": true,
	"XDocument": true, "XElement": true, "XAttribute": true, "XName": true, "XNamespace": true,
	"XNode": true, "XText": true, "XComment": true, "XProcessingInstruction": true, "XDeclaration": true,
	"XDocumentType": true, "XObject": true, "XStreamingElement": true,
	"XContainer": true,
	// System.Data / System.Data.Common
	"DataTable": true, "DataSet": true, "DataRow": true, "DataColumn": true, "DataView": true,
	"DbConnection": true, "DbCommand": true, "DbDataReader": true, "DbDataAdapter": true,
	"DbParameter": true, "DbTransaction": true, "CommandType": true,
	"SqlConnection": true, "SqlCommand": true, "SqlDataReader": true, "SqlDataAdapter": true,
	"SqlParameter": true, "SqlTransaction": true,
	// System.Security.Cryptography
	"MD5": true, "SHA1": true, "SHA256": true, "SHA384": true, "SHA512": true,
	"RSA": true, "DSA": true, "ECDsa": true, "ECDiffieHellman": true,
	"Aes": true, "DES": true, "TripleDES": true, "RC2": true,
	"HMACSHA256": true, "HMACSHA512": true, "HMACMD5": true,
	"RandomNumberGenerator": true, "ProtectedData": true, "ProtectedMemory": true,
	"CryptoStream": true, "FromBase64String": true, "ToBase64String": true,
	"HashAlgorithm": true, "SymmetricAlgorithm": true, "AsymmetricAlgorithm": true,
	"X509Certificate": true, "X509Certificate2": true, "X509Store": true, "X509Chain": true,
	// System.Reflection
	"Assembly": true, "AssemblyName": true, "Module": true, "ConstructorInfo": true,
	"MethodInfo": true, "PropertyInfo": true, "FieldInfo": true, "EventInfo": true,
	"ParameterInfo": true, "MemberInfo": true, "TypeInfo": true,
	"BindingFlags": true, "Binder": true, "CallingConventions": true,
	"AssemblyBuilder": true, "ModuleBuilder": true,
	"ConstructorBuilder": true, "MethodBuilder": true, "PropertyBuilder": true,
	"ILGenerator": true, "OpCodes": true, "Emit": true,
	"CustomAttributeData": true, "CustomAttributeNamedArgument": true, "CustomAttributeTypedArgument": true,
	"AssemblyLoadContext": true, "MetadataLoadContext": true,
	// System.Diagnostics
	"Process": true, "ProcessStartInfo": true, "Stopwatch": true, "Trace": true, "Debug": true,
	"TraceSource": true, "TraceListener": true, "DefaultTraceListener": true, "ConsoleTraceListener": true,
	"TextWriterTraceListener": true, "EventLog": true, "PerformanceCounter": true,
	"FileVersionInfo": true, "StackFrame": true, "StackTrace": true, "Debugger": true,
	"ProcessModule": true, "ProcessThread": true,
	// System.ComponentModel
	"INotifyPropertyChanged": true, "PropertyChangedEventArgs": true, "PropertyChangedEventHandler": true,
	"INotifyDataErrorInfo": true, "DataErrorsChangedEventArgs": true,
	"TypeConverter": true, "TypeDescriptor": true,
	"BackgroundWorker": true, "AsyncOperation": true, "AsyncOperationManager": true,
	"Component": true, "Container": true,
	// 更多常用类型
	"HttpResponse": true, "ActionResult": true, "IActionResult": true,
	"Controller": true, "ControllerBase": true, "ApiController": true,
	"Route": true, "HttpGet": true, "HttpPost": true, "HttpPut": true, "HttpDelete": true, "HttpPatch": true,
	"FromBody": true, "FromQuery": true, "FromRoute": true, "FromForm": true, "FromHeader": true, "FromServices": true,
	"BindProperty": true, "BindNever": true, "BindRequired": true,
	"AllowAnonymous": true, "Authorize": true, "Authenticate": true,
	"ServiceCollection": true, "ServiceProvider": true, "AddSingleton": true, "AddScoped": true, "AddTransient": true,
	"BuildServiceProvider": true, "GetRequiredService": true, "GetService": true,
	"IConfiguration": true, "IConfigurationBuilder": true, "ConfigurationBuilder": true,
	"AddJsonFile": true, "AddEnvironmentVariables": true, "Build": true,
	"ILogger": true, "ILoggerFactory": true, "LoggerFactory": true,
	"LogInformation": true, "LogWarning": true, "LogError": true, "LogDebug": true, "LogCritical": true,
	"WebApplication": true, "WebApplicationBuilder": true, "CreateBuilder": true,
	"MapGet": true, "MapPost": true, "MapPut": true, "MapDelete": true, "MapPatch": true,
	"UseAuthorization": true, "UseAuthentication": true, "UseCors": true, "UseStaticFiles": true,
	"UseRouting": true, "UseEndpoints": true, "UseSwagger": true, "UseHttpsRedirection": true,
	"Services": true, "Configuration": true,
	"EntityFrameworkCore": true, "DbContext": true, "DbSet": true, "OnModelCreating": true, "ModelBuilder": true,
	"Entity": true, "HasKey": true, "HasIndex": true, "Property": true, "IsRequired": true, "HasMaxLength": true,
	"ToTable": true, "HasColumnName": true, "HasColumnType": true, "Ignore": true,
	"HasOne": true, "HasMany": true, "WithOne": true, "WithMany": true, "HasForeignKey": true,
	"HasData": true, "ValueGeneratedOnAdd": true, "UseIdentityColumn": true,
	"SaveChanges": true, "SaveChangesAsync": true, "Find": true, "Add": true, "Update": true, "Remove": true,
	"AddRange": true, "UpdateRange": true, "RemoveRange": true, "ToListAsync": true,
	"FirstOrDefaultAsync": true, "SingleOrDefaultAsync": true, "AnyAsync": true, "AllAsync": true, "CountAsync": true,
	"ContainsAsync": true, "Include": true, "ThenInclude": true,
	// Object 常用方法
	"ToString": true, "Equals": true, "GetHashCode": true, "GetType": true, "ReferenceEquals": true,
	"CompareTo": true, "AsSpan": true, "AsMemory": true, "TryParse": true, "Parse": true,
}

func isCSKeyword(name string) bool {
	return csKeywords[name]
}
