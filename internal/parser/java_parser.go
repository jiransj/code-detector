package parser

import (
	"regexp"
	"strings"

	"code-detector/internal/model"
)

// JavaParser 解析 Java/Kotlin 源文件
type JavaParser struct {
	language string // "java" 或 "kotlin"
}

func NewJavaParser() *JavaParser     { return &JavaParser{language: "java"} }
func NewKotlinParser() *JavaParser   { return &JavaParser{language: "kotlin"} }

func (p *JavaParser) Language() string { return p.language }

func (p *JavaParser) Globals(filePath string, content []byte) ([]*model.GlobalVariable, error) {
	text := string(content)
	lines := strings.Split(text, "\n")
	commentMask := makeCommentMask(lines, []string{"//"}, [][2]string{{"/*", "*/"}})

	var vars []*model.GlobalVariable
	// 匹配 public static Type NAME = value; 等静态字段
	staticRegex := regexp.MustCompile(`(?:public|private|protected|static|final)\s+(?:static\s+)?(?:(?P<type>\w+(?:\<[^\>]*\>)?(?:\[\])*))\s+(?P<name>\w+)\s*(?:=|;)`)

	for i, line := range lines {
		if commentMask[i] {
			continue
		}
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.HasPrefix(trimmed, "//") || strings.HasPrefix(trimmed, "/*") {
			continue
		}
		matches := staticRegex.FindStringSubmatch(line)
		if matches == nil {
			continue
		}
		nameIdx := staticRegex.SubexpIndex("name")
		typeIdx := staticRegex.SubexpIndex("type")
		name := matches[nameIdx]
		varType := ""
		if typeIdx >= 0 && typeIdx < len(matches) {
			varType = matches[typeIdx]
		}
		if name != "" {
			vars = append(vars, &model.GlobalVariable{
				Name: name, VarType: varType, Language: p.language,
				FilePath: filePath, LineNum: i + 1,
				IsConst: strings.Contains(trimmed, "final"),
			})
		}
	}
	return vars, nil
}

// javaFuncRegex 匹配方法定义
var javaFuncRegex = regexp.MustCompile(
	`(?:(?:public|private|protected|static|final|abstract|synchronized|native|transient|volatile|strictfp|default)\s+)*(?:\w+(?:\[\])*(?:\<[^\>]*\>)?)\s+(?P<name>\w+)\s*\(`,
)

// kotlinFuncRegex 匹配 Kotlin fun 定义
var kotlinFuncRegex = regexp.MustCompile(
	`(?:fun\s+)(?P<name>\w+)\s*\(`,
)

// javaCallRegex 匹配 Java 方法调用
var javaCallRegex = regexp.MustCompile(`(?:(\w+)\.)?(\w+)\s*\(`)

func (p *JavaParser) Parse(filePath string, content []byte) ([]*model.Function, error) {
	fl := NewFileLinesFromBytes(content)
	lines := fl.Lines()
	text := fl.Text()

	commentMask := makeCommentMask(lines, []string{"//"}, [][2]string{{"/*", "*/"}})
	stringMask := makeStringMask(lines)

	funcRegex := javaFuncRegex
	if p.language == "kotlin" {
		funcRegex = kotlinFuncRegex
	}

	type javaFuncStart struct {
		lineIdx int
		name    string
	}
	var starts []javaFuncStart

	for i, line := range lines {
		if commentMask[i] || stringMask[i] {
			continue
		}
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			continue
		}
		// 排除注解行、接口方法、抽象方法（没有方法体）
		if strings.HasPrefix(trimmed, "@") {
			continue
		}

		matches := funcRegex.FindStringSubmatch(line)
		if matches == nil {
			continue
		}
		nameIdx := funcRegex.SubexpIndex("name")
		if nameIdx < 0 || nameIdx >= len(matches) {
			continue
		}
		name := matches[nameIdx]
		if name == "" {
			continue
		}

		starts = append(starts, javaFuncStart{lineIdx: i, name: name})
	}

	if len(starts) == 0 {
		return nil, nil
	}

	allFuncs := make([]*model.Function, 0, len(starts))

	for _, fs := range starts {
		// 找到第一个 '{' 开始匹配
		offset := fl.LineOffset(fs.lineIdx)
		line := lines[fs.lineIdx]
		braceIdx := strings.Index(line, "{")
		if braceIdx < 0 {
			// 查找后续行的 {
			found := false
			for j := fs.lineIdx + 1; j < len(lines); j++ {
				if commentMask[j] {
					continue
				}
				if idx := strings.Index(lines[j], "{"); idx >= 0 {
					braceIdx = idx
					offset = fl.LineOffset(j) + idx
					found = true
					break
				}
			}
			if !found {
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

		callStats := extractCallStats(body, javaCallRegex, stringMask, commentMask, startLine, endLine, isJavaKeyword, nil)

		f := &model.Function{
			Name:         fs.name,
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

// javaKeywords 是 Java/Kotlin 关键字、常用标准库类和方法的集合（map 数据驱动）
var javaKeywords = map[string]bool{
	// Java 语言关键字
	"if": true, "else": true, "for": true, "while": true, "do": true,
	"switch": true, "case": true, "default": true, "break": true,
	"continue": true, "return": true, "throw": true, "try": true,
	"catch": true, "finally": true, "new": true, "this": true,
	"super": true, "instanceof": true, "synchronized": true,
	"abstract": true, "assert": true, "boolean": true, "byte": true,
	"char": true, "class": true, "const": true, "double": true,
	"enum": true, "exports": true, "extends": true, "final": true,
	"float": true, "goto": true, "implements": true, "import": true,
	"int": true, "interface": true, "long": true, "module": true,
	"native": true, "open": true, "opens": true, "package": true,
	"private": true, "protected": true, "provides": true, "public": true,
	"record": true, "requires": true, "short": true, "static": true,
	"strictfp": true, "to": true, "transient": true, "transitive": true,
	"uses": true, "void": true, "volatile": true, "with": true,
	"var": true, "yield": true, "sealed": true, "permits": true,
	"null": true, "true": true, "false": true,
	// java.lang.*
	"Object": true, "Class": true, "String": true, "StringBuilder": true,
	"StringBuffer": true, "System": true, "Runtime": true, "Thread": true,
	"Runnable": true, "Comparable": true, "Cloneable": true,
	"CharSequence": true, "Number": true, "Integer": true, "Long": true,
	"Double": true, "Float": true, "Boolean": true, "Byte": true,
	"Short": true, "Character": true, "Void": true, "Math": true,
	"StrictMath": true, "Enum": true, "Throwable": true, "Exception": true,
	"RuntimeException": true, "Error": true, "StackTraceElement": true,
	"ThreadLocal": true, "InheritableThreadLocal": true,
	"Package": true, "Module": true, "Record": true,
	"Process": true, "ProcessBuilder": true, "ShutdownHook": true,
	"AutoCloseable": true, "Appendable": true, "Readable": true,
	"ReflectiveOperationException": true,
	// java.lang reflect
	"Field": true, "Method": true, "Constructor": true, "Array": true,
	"InvocationHandler": true, "Modifier": true,
	"AccessibleObject": true, "ParameterizedType": true,
	// java.lang.annotation
	"Annotation": true, "Retention": true, "Target": true, "Documented": true,
	"Inherited": true, "Repeatable": true, "RetentionPolicy": true,
	"ElementType": true, "SuppressWarnings": true, "Deprecated": true,
	"Override": true, "FunctionalInterface": true, "SafeVarargs": true,
	// java.util.*
	"ArrayList": true, "LinkedList": true, "HashMap": true, "LinkedHashMap": true,
	"TreeMap": true, "HashSet": true, "LinkedHashSet": true, "TreeSet": true,
	"Vector": true, "Stack": true, "Hashtable": true, "Properties": true,
	"Collections": true, "Arrays": true, "Objects": true, "Optional": true,
	"OptionalInt": true, "OptionalLong": true, "OptionalDouble": true,
	"Random": true, "UUID": true, "Date": true,
	"Calendar": true, "GregorianCalendar": true, "TimeZone": true,
	"Locale": true, "Currency": true, "BitSet": true, "Timer": true,
	"TimerTask": true, "Iterator": true, "ListIterator": true, "Iterable": true,
	"Collection": true, "List": true, "Set": true, "Map": true, "Queue": true,
	"Deque": true, "SortedSet": true, "SortedMap": true, "NavigableSet": true,
	"NavigableMap": true, "Comparator": true, "Enumeration": true,
	"EventListener": true, "EventObject": true, "Observer": true,
	"StringTokenizer": true, "Scanner": true,
	"AbstractList": true, "AbstractSet": true, "AbstractMap": true,
	"AbstractCollection": true, "AbstractQueue": true, "PriorityQueue": true,
	"ArrayDeque": true, "EnumMap": true, "EnumSet": true,
	"IdentityHashMap": true, "WeakHashMap": true,
	"ConcurrentHashMap": true, "ConcurrentLinkedQueue": true,
	"ConcurrentLinkedDeque": true, "CopyOnWriteArrayList": true,
	"CopyOnWriteArraySet": true, "PriorityBlockingQueue": true,
	"DelayQueue": true, "SynchronousQueue": true, "LinkedBlockingQueue": true,
	"LinkedBlockingDeque": true, "ArrayBlockingQueue": true,
	"CountDownLatch": true, "CyclicBarrier": true, "Semaphore": true,
	"Exchanger": true, "Phaser": true, "Executor": true, "Executors": true,
	"ExecutorService": true, "ScheduledExecutorService": true,
	"ThreadFactory": true, "ThreadPoolExecutor": true, "ScheduledThreadPoolExecutor": true,
	"Future": true, "Callable": true, "CompletionService": true,
	"ExecutorCompletionService": true, "CompletableFuture": true,
	"ForkJoinPool": true, "ForkJoinTask": true, "RecursiveTask": true,
	"RecursiveAction": true, "AtomicInteger": true, "AtomicLong": true,
	"AtomicBoolean": true, "AtomicReference": true, "LongAdder": true,
	"DoubleAdder": true, "LongAccumulator": true, "DoubleAccumulator": true,
	"ReentrantLock": true, "ReentrantReadWriteLock": true, "Lock": true,
	"ReadWriteLock": true, "Condition": true, "LockSupport": true,
	"AbstractQueuedSynchronizer": true,
	"Stream": true, "IntStream": true, "LongStream": true, "DoubleStream": true,
	"Collectors": true, "Collector": true, "StreamSupport": true,
	"SummaryStatistics": true, "IntSummaryStatistics": true,
	"LongSummaryStatistics": true, "DoubleSummaryStatistics": true,
	"Spliterator": true, "Spliterators": true,
	"function": true,
	"Supplier": true, "UnaryOperator": true, "BinaryOperator": true,
	// java.util.regex
	"Pattern": true, "Matcher": true, "MatchResult": true,
	// java.util.logging
	"Logger": true, "Level": true, "LogManager": true, "LogRecord": true,
	"Handler": true, "ConsoleHandler": true, "FileHandler": true,
	"Formatter": true, "SimpleFormatter": true, "XMLFormatter": true,
	"Filter": true, "MemoryHandler": true, "SocketHandler": true,
	// java.time.* (Java 8+)
	"LocalDate": true, "LocalTime": true, "LocalDateTime": true,
	"ZonedDateTime": true, "OffsetDateTime": true, "OffsetTime": true,
	"Instant": true, "Duration": true, "Period": true, "Year": true,
	"YearMonth": true, "MonthDay": true, "DayOfWeek": true, "Month": true,
	"ZoneId": true, "ZoneOffset": true, "ZoneRules": true,
	"DateTimeFormatter": true, "DateTimeFormatterBuilder": true,
	"Temporal": true, "TemporalAccessor": true, "TemporalAdjuster": true,
	"TemporalAdjusters": true, "TemporalAmount": true, "TemporalUnit": true,
	"ChronoUnit": true, "ChronoField": true, "Chronology": true,
	"Clock": true,
	// java.io.*
	"File": true, "FileDescriptor": true, "FilePermission": true,
	"FileInputStream": true, "FileOutputStream": true, "FileReader": true,
	"FileWriter": true, "BufferedInputStream": true, "BufferedOutputStream": true,
	"BufferedReader": true, "BufferedWriter": true, "FilterInputStream": true,
	"FilterOutputStream": true, "DataInputStream": true, "DataOutputStream": true,
	"ObjectInputStream": true, "ObjectOutputStream": true,
	"InputStreamReader": true, "OutputStreamWriter": true,
	"PrintStream": true, "PrintWriter": true, "StringReader": true,
	"StringWriter": true, "CharArrayReader": true, "CharArrayWriter": true,
	"PipedInputStream": true, "PipedOutputStream": true,
	"PipedReader": true, "PipedWriter": true, "ByteArrayInputStream": true,
	"ByteArrayOutputStream": true, "SequenceInputStream": true,
	"PushbackInputStream": true, "PushbackReader": true,
	"LineNumberReader": true, "LineNumberInputStream": true,
	"StreamTokenizer": true, "RandomAccessFile": true,
	"Serializable": true, "Externalizable": true, "ObjectStreamField": true,
	"ObjectStreamClass": true, "ObjectInputFilter": true,
	"Console": true,
	// java.nio.*
	"ByteBuffer": true, "CharBuffer": true, "ShortBuffer": true,
	"IntBuffer": true, "LongBuffer": true, "FloatBuffer": true,
	"DoubleBuffer": true, "MappedByteBuffer": true,
	"Buffer": true, "BufferOverflowException": true,
	"BufferUnderflowException": true,
	"Channels": true, "FileChannel": true, "SocketChannel": true,
	"ServerSocketChannel": true, "DatagramChannel": true,
	"Selector": true, "SelectionKey": true, "SelectableChannel": true,
	"Path": true, "Paths": true, "Files": true, "FileSystem": true,
	"FileSystems": true, "FileStore": true, "DirectoryStream": true,
	"PathMatcher": true, "WatchService": true, "WatchKey": true,
	"WatchEvent": true, "FileVisitor": true, "SimpleFileVisitor": true,
	"FileVisitResult": true, "OpenOption": true, "StandardOpenOption": true,
	"CopyOption": true, "StandardCopyOption": true, "LinkOption": true,
	"FileAttribute": true, "PosixFilePermission": true,
	"PosixFileAttributes": true, "PosixFileAttributeView": true,
	"AclFileAttributeView": true, "BasicFileAttributes": true,
	"BasicFileAttributeView": true, "FileTime": true,
	"Charset": true, "StandardCharsets": true, "CoderResult": true,
	"CharsetEncoder": true, "CharsetDecoder": true,
	// java.net.*
	"URL": true, "URI": true, "URLConnection": true, "HttpURLConnection": true,
	"URLEncoder": true, "URLDecoder": true, "Socket": true,
	"ServerSocket": true, "DatagramSocket": true, "DatagramPacket": true,
	"InetAddress": true, "InetSocketAddress": true, "Inet4Address": true,
	"Inet6Address": true, "NetworkInterface": true, "InterfaceAddress": true,
	"Proxy": true, "ProxySelector": true, "Authenticator": true,
	"PasswordAuthentication": true, "CookieManager": true,
	"CookieStore": true, "HttpCookie": true,
	"MulticastSocket": true, "StandardSocketOptions": true,
	"SocketOptions": true, "SocketAddress": true, "SocketPermission": true,
	"ContentHandler": true, "URLStreamHandler": true,
	// java.sql.*
	"Connection": true, "Statement": true, "PreparedStatement": true,
	"CallableStatement": true, "ResultSet": true, "ResultSetMetaData": true,
	"DriverManager": true, "Driver": true, "DatabaseMetaData": true,
	"SQLException": true, "SQLWarning": true, "SQLData": true,
	"SQLInput": true, "SQLOutput": true, "Savepoint": true,
	"Blob": true, "Clob": true, "NClob": true,
	"Ref": true, "RowId": true, "Struct": true, "SQLXML": true,
	"Types": true, "Time": true, "Timestamp": true,
	"BatchUpdateException": true, "DataSource": true,
	// javax.sql.*
	"RowSet": true, "RowSetMetaData": true, "RowSetEvent": true,
	"RowSetListener": true, "RowSetInternal": true, "RowSetReader": true,
	"RowSetWriter": true, "CachedRowSet": true, "FilteredRowSet": true,
	"JdbcRowSet": true, "JoinRowSet": true, "WebRowSet": true,
	"PooledConnection": true, "ConnectionPoolDataSource": true,
	"ConnectionEvent": true, "ConnectionEventListener": true,
	"XAConnection": true, "XADataSource": true, "XAResource": true,
	// java.math.*
	"BigInteger": true, "BigDecimal": true, "MathContext": true,
	"RoundingMode": true,
	// java.text.*
	"DateFormat": true, "SimpleDateFormat": true, "DateFormatSymbols": true,
	"DecimalFormat": true, "DecimalFormatSymbols": true,
	"NumberFormat": true, "ChoiceFormat": true, "MessageFormat": true,
	"Collator": true, "CollationKey": true, "BreakIterator": true,
	"Normalizer": true, "StringCharacterIterator": true,
	// java.security.*
	"MessageDigest": true, "Signature": true, "KeyPairGenerator": true,
	"KeyFactory": true, "KeyStore": true, "CertificateFactory": true,
	"KeyPair": true, "PublicKey": true, "PrivateKey": true, "Key": true,
	"SecureRandom": true, "AlgorithmParameters": true,
	"AlgorithmParameterGenerator": true, "CodeSigner": true,
	"CodeSource": true, "DigestInputStream": true, "DigestOutputStream": true,
	"DomCrypto": true, "GeneralSecurityException": true,
	"GuardedObject": true, "Guard": true, "Identity": true,
	"IdentityScope": true, "InvalidKeyException": true,
	"InvalidParameterException": true, "NoSuchAlgorithmException": true,
	"Permission": true, "Permissions": true, "Policy": true,
	"Principal": true, "PrivilegedAction": true, "Provider": true,
	"ProviderException": true, "Security": true, "SecurityPermission": true,
	"Signer": true, "UnrecoverableEntryException": true,
	"UnrecoverableKeyException": true,
	// javax.crypto.*
	"Cipher": true, "CipherInputStream": true, "CipherOutputStream": true,
	"KeyGenerator": true, "SecretKey": true, "SecretKeyFactory": true,
	"Mac": true, "SealedObject": true, "ExemptionMechanism": true,
	"EncryptedPrivateKeyInfo": true, "AEADBadTagException": true,
	"NoSuchPaddingException": true, "BadPaddingException": true,
	"IllegalBlockSizeException": true, "ShortBufferException": true,
	"KeyAgreement": true,
	// java.xml / javax.xml.*
	"SAXException": true, "SAXParseException": true, "SAXReader": true,
	"SAXParser": true, "SAXParserFactory": true, "XMLReader": true,
	"InputSource": true, "Attributes": true, "DefaultHandler": true,
	"DocumentBuilder": true, "DocumentBuilderFactory": true, "Document": true,
	"Element": true, "Node": true, "NodeList": true, "NamedNodeMap": true,
	"Attr": true, "Text": true, "Comment": true, "CDATASection": true,
	"ProcessingInstruction": true, "Entity": true, "EntityReference": true,
	"DocumentType": true, "DOMImplementation": true, "Transformer": true,
	"TransformerFactory": true, "TransformerException": true,
	"TransformerConfigurationException": true, "Source": true,
	"Result": true, "StreamSource": true, "StreamResult": true,
	"DOMSource": true, "DOMResult": true, "SAXSource": true, "SAXResult": true,
	"StAXSource": true, "StAXResult": true, "XMLEventReader": true,
	"XMLEventWriter": true, "XMLInputFactory": true, "XMLOutputFactory": true,
	"XMLEvent": true, "StartElement": true, "EndElement": true,
	"Characters": true, "Attribute": true, "Namespace": true,
	"QName": true, "XMLStreamReader": true, "XMLStreamWriter": true,
	"XPath": true, "XPathFactory": true, "XPathExpression": true,
	"XPathExpressionException": true, "XPathConstants": true,
	// javax.annotation.*
	"PostConstruct": true, "PreDestroy": true, "Resource": true,
	"Resources": true, "Generated": true,
	// Kotlin 标准库
	"BooleanArray": true, "ByteArray": true, "CharArray": true,
	"DoubleArray": true, "FloatArray": true, "IntArray": true,
	"LongArray": true, "ShortArray": true,
	"listOf": true, "mutableListOf": true, "setOf": true, "mutableSetOf": true,
	"mapOf": true, "mutableMapOf": true, "arrayOf": true,
	"intArrayOf": true, "longArrayOf": true, "doubleArrayOf": true,
	"booleanArrayOf": true, "byteArrayOf": true, "charArrayOf": true,
	"shortArrayOf": true, "floatArrayOf": true,
	"emptyList": true, "emptySet": true, "emptyMap": true,
	"listOfNotNull": true, "hashMapOf": true, "linkedMapOf": true,
	"hashSetOf": true, "linkedSetOf": true, "sortedSetOf": true,
	"sortedMapOf": true, "lazy": true, "lazyOf": true,
	"sequenceOf": true, "generateSequence": true, "emptySequence": true,
	"print": true, "println": true, "printf": true,
	"readLine": true, "compareBy": true, "thenBy": true, "compareByDescending": true,
	"apply": true, "also": true, "let": true, "run": true,
	"takeIf": true, "takeUnless": true, "repeat": true,
	"use": true, "useLines": true, "checkNotNull": true,
	"require": true, "requireNotNull": true,
	"check": true, "error": true, "TODO": true,
	"Any": true, "Unit": true, "Nothing": true,
	"Pair": true, "Triple": true,
	// 常用日志 / 测试 / 框架
	"LoggerFactory": true,
	"log": true, "LOG": true, "logger": true,
	"Test": true, "BeforeEach": true, "BeforeAll": true, "AfterEach": true,
	"AfterAll": true, "DisplayName": true, "Disabled": true, "Nested": true,
	"Tag": true, "Timeout": true, "TestMethodOrder": true, "Order": true,
	"ParameterizedTest": true, "ValueSource": true, "CsvSource": true,
	"MethodSource": true, "Arguments": true, "ArgumentsSource": true,
	"Mock": true, "Spy": true, "Captor": true, "InjectMocks": true,
	"Mockito": true, "when": true, "verify": true, "doReturn": true,
	"doThrow": true, "doAnswer": true, "thenReturn": true,
	"thenThrow": true, "thenAnswer": true, "any": true, "eq": true,
	"assertEquals": true, "assertNotEquals": true,
	"assertTrue": true, "assertFalse": true, "assertNull": true,
	"assertNotNull": true, "assertSame": true, "assertNotSame": true,
	"assertThrows": true, "assertTimeout": true, "assertArrayEquals": true,
	"assertIterableEquals": true, "assertLinesMatch": true,
	"fail": true, "assumeTrue": true, "assumeFalse": true,
	"SpringBootTest": true, "SpringRunner": true, "SpringJUnit4ClassRunner": true,
	"Autowired": true, "Qualifier": true, "Value": true,
	"Component": true, "Service": true, "Repository": true,
	"Controller": true, "RestController": true, "RequestMapping": true,
	"GetMapping": true, "PostMapping": true, "PutMapping": true,
	"DeleteMapping": true, "PatchMapping": true, "PathVariable": true,
	"RequestParam": true, "RequestBody": true, "ResponseBody": true,
	"ResponseStatus": true, "ExceptionHandler": true, "ControllerAdvice": true,
	"EnableAutoConfiguration": true, "SpringBootApplication": true,
	"Configuration": true, "Bean": true, "Scope": true, "Lazy": true,
	"Primary": true, "Profile": true, "Conditional": true,
	"ConfigurationProperties": true, "EnableConfigurationProperties": true,
	"Table": true, "Column": true, "Id": true,
	"GeneratedValue": true, "GenerationType": true, "SequenceGenerator": true,
	"TableGenerator": true, "Basic": true, "Transient": true,
	"OneToOne": true, "OneToMany": true, "ManyToOne": true, "ManyToMany": true,
	"JoinColumn": true, "JoinTable": true, "CascadeType": true,
	"FetchType": true, "CrudRepository": true,
	"JpaRepository": true, "PagingAndSortingRepository": true,
	"Query": true, "Modifying": true, "Param": true,
	"Transactional": true,
	"JsonIgnore": true, "JsonFormat": true, "JsonProperty": true,
	"JsonInclude": true, "JsonAlias": true, "JsonTypeInfo": true,
	"JsonSubTypes": true, "JsonCreator": true, "JsonValue": true,
	"JsonRawValue": true, "JsonIgnoreProperties": true,
	"Nullable": true, "NonNull": true, "Nonnull": true,
	"Size": true, "NotNull": true, "NotEmpty": true, "NotBlank": true,
	"Min": true, "Max": true, "Email": true,
	"Positive": true, "PositiveOrZero": true, "Negative": true,
	"NegativeOrZero": true, "Past": true,
	"AssertTrue": true, "AssertFalse": true, "Valid": true, "Validated": true,
	"Log4j2": true, "CommonsLog": true, "Logback": true,
	"Getter": true, "Setter": true, "Data": true, "NoArgsConstructor": true,
	"AllArgsConstructor": true, "RequiredArgsConstructor": true,
	"ToString": true, "EqualsAndHashCode": true, "Builder": true,
	"Accessors": true, "FieldDefaults": true,
	"Singular": true, "Delegate": true, "UtilityClass": true,
	"Cleanup": true, "SneakyThrows": true,
	"Log": true, "Slf4j": true, "CustomLog": true, "XSlf4j": true,
	"Flogger": true, "Jacksonized": true, "SuperBuilder": true,
	"With": true, "Wither": true,
}

func isJavaKeyword(name string) bool {
	return javaKeywords[name]
}
