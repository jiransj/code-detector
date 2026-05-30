package ast_stress

// ────────────────────────────────────────────────────────────
// 巨型 Go 文件压力测试 (1000+ 函数)
// 目标: 测试 tree-sitter AST 解析器在高函数密度下的
//       性能、内存和正确性
// ────────────────────────────────────────────────────────────

import (
	"context"
	"fmt"
	"sync"
)

// ── 泛型辅助类型 ─────────────────────────────────────────

type Stack[T any] struct {
	items []T
	mu    sync.RWMutex
}

func NewStack[T any]() *Stack[T] {
	return &Stack[T]{items: make([]T, 0, 128)}
}

func (s *Stack[T]) Push(v T) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.items = append(s.items, v)
}

func (s *Stack[T]) Pop() (T, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if len(s.items) == 0 {
		var zero T
		return zero, false
	}
	v := s.items[len(s.items)-1]
	s.items = s.items[:len(s.items)-1]
	return v, true
}

type Pair[A, B any] struct {
	First  A
	Second B
}

func MakePair[A, B any](a A, b B) Pair[A, B] {
	return Pair[A, B]{First: a, Second: b}
}

// ── 自动生成 200 个简单函数 ───────────────────────────

func Fn0001() string { return fmt.Sprintf("fn%04d", 1) }
func Fn0002() string { return fmt.Sprintf("fn%04d", 2) }
func Fn0003() string { return fmt.Sprintf("fn%04d", 3) }
func Fn0004() string { return fmt.Sprintf("fn%04d", 4) }
func Fn0005() string { return fmt.Sprintf("fn%04d", 5) }
func Fn0006() string { return fmt.Sprintf("fn%04d", 6) }
func Fn0007() string { return fmt.Sprintf("fn%04d", 7) }
func Fn0008() string { return fmt.Sprintf("fn%04d", 8) }
func Fn0009() string { return fmt.Sprintf("fn%04d", 9) }
func Fn0010() string { return fmt.Sprintf("fn%04d", 10) }
func Fn0011() string { return fmt.Sprintf("fn%04d", 11) }
func Fn0012() string { return fmt.Sprintf("fn%04d", 12) }
func Fn0013() string { return fmt.Sprintf("fn%04d", 13) }
func Fn0014() string { return fmt.Sprintf("fn%04d", 14) }
func Fn0015() string { return fmt.Sprintf("fn%04d", 15) }
func Fn0016() string { return fmt.Sprintf("fn%04d", 16) }
func Fn0017() string { return fmt.Sprintf("fn%04d", 17) }
func Fn0018() string { return fmt.Sprintf("fn%04d", 18) }
func Fn0019() string { return fmt.Sprintf("fn%04d", 19) }
func Fn0020() string { return fmt.Sprintf("fn%04d", 20) }

func Fn0021() string { return Fn0001() + Fn0002() }
func Fn0022() string { return Fn0003() + Fn0004() }
func Fn0023() string { return Fn0005() + Fn0006() }
func Fn0024() string { return Fn0007() + Fn0008() }
func Fn0025() string { return Fn0009() + Fn0010() }
func Fn0026() string { return Fn0011() + Fn0012() }
func Fn0027() string { return Fn0013() + Fn0014() }
func Fn0028() string { return Fn0015() + Fn0016() }
func Fn0029() string { return Fn0017() + Fn0018() }
func Fn0030() string { return Fn0019() + Fn0020() }

func Fn0031() string { return fmt.Sprintf("%s-%s", Fn0021(), Fn0022()) }
func Fn0032() string { return fmt.Sprintf("%s-%s", Fn0023(), Fn0024()) }
func Fn0033() string { return fmt.Sprintf("%s-%s", Fn0025(), Fn0026()) }
func Fn0034() string { return fmt.Sprintf("%s-%s", Fn0027(), Fn0028()) }
func Fn0035() string { return fmt.Sprintf("%s-%s", Fn0029(), Fn0030()) }

func Fn0036() string { return Fn0031() + Fn0032() + Fn0033() }
func Fn0037() string { return Fn0034() + Fn0035() }
func Fn0038() string { return fmt.Sprintf("%d", len(Fn0036()+Fn0037())) }

func Fn0039() string { return Fn0001() }
func Fn0040() string { return Fn0002() }

// ── 带复杂参数和返回值的函数 ─────────────────────────

func Fn0041(ctx context.Context, names []string, opts map[string]interface{}) (map[string]int, error) {
	result := make(map[string]int, len(names))
	for _, name := range names {
		if v, ok := opts[name]; ok {
			switch val := v.(type) {
			case int:
				result[name] = val * 2
			case float64:
				result[name] = int(val) + 1
			case string:
				result[name] = len(val)
			case []byte:
				result[name] = len(val) / 2
			default:
				result[name] = 0
			}
		} else {
			result[name] = -1
		}
	}
	return result, ctx.Err()
}

func Fn0042[T comparable](items []T, pred func(T) bool) (matched []T, count int) {
	matched = make([]T, 0, len(items)/2)
	for _, item := range items {
		if pred(item) {
			matched = append(matched, item)
			count++
		}
	}
	return matched, count
}

func Fn0043[K comparable, V any](m map[K]V, keys []K) []V {
	results := make([]V, 0, len(keys))
	for _, k := range keys {
		if v, ok := m[k]; ok {
			results = append(results, v)
		}
	}
	return results
}

type Numeric interface {
	~int | ~int8 | ~int16 | ~int32 | ~int64 | ~uint | ~uint8 | ~uint16 | ~uint32 | ~uint64 | ~float32 | ~float64
}

func Fn0044[T Numeric](a, b T) T {
	if a > b {
		return a
	}
	return b
}

func Fn0045[T Numeric](values []T) (T, T) {
	if len(values) == 0 {
		var zero T
		return zero, zero
	}
	min, max := values[0], values[0]
	for _, v := range values[1:] {
		if v < min {
			min = v
		}
		if v > max {
			max = v
		}
	}
	return min, max
}

// ── 深度嵌套的条件分支（测试圈复杂度） ───────────────

func Fn0046(a, b, c, d, e, f int) int {
	if a > 0 {
		if b > 0 {
			if c > 0 {
				if d > 0 {
					if e > 0 {
						return a + b + c + d + e + f
					} else if f > 0 {
						return a + b + c + d + f
					} else {
						return a + b + c + d
					}
				} else {
					return a + b + c
				}
			} else {
				return a + b
			}
		} else {
			return a
		}
	}
	return 0
}

func Fn0047(items []int) []int {
	result := make([]int, 0, len(items))
outer:
	for i, x := range items {
		for j, y := range items {
			if i == j {
				continue
			}
			if x == y {
				continue outer
			}
		}
		result = append(result, x)
	}
	return result
}

// ── 通道 + select 模式 ────────────────────────────────

type Result[T any] struct {
	Value T
	Err   error
}

func Fn0048[T any](ctx context.Context, ch <-chan T, timeout int) (Result[T], error) {
	select {
	case val, ok := <-ch:
		if !ok {
			return Result[T]{}, fmt.Errorf("channel closed")
		}
		return Result[T]{Value: val}, nil
	case <-ctx.Done():
		return Result[T]{}, ctx.Err()
	default:
		return Fn0048Timeout[T](timeout)
	}
}

func Fn0048Timeout[T any](timeout int) (Result[T], error) {
	if timeout <= 0 {
		return Result[T]{}, fmt.Errorf("timeout must be positive, got %d", timeout)
	}
	var zero T
	return Result[T]{Value: zero}, nil
}

// ── 方法 + receiver 测试 ──────────────────────────────

type Service struct {
	name    string
	cache   *sync.Map
	clients map[string]interface{}
	mu      sync.RWMutex
}

func NewService(name string) *Service {
	return &Service{
		name:    name,
		cache:   &sync.Map{},
		clients: make(map[string]interface{}),
	}
}

func (s *Service) GetName() string {
	return s.name
}

func (s *Service) SetCache(key string, value interface{}) {
	s.cache.Store(key, value)
}

func (s *Service) GetCache(key string) (interface{}, bool) {
	return s.cache.Load(key)
}

func (s *Service) RegisterClient(id string, info interface{}) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, exists := s.clients[id]; exists {
		return fmt.Errorf("client %s already registered", id)
	}
	s.clients[id] = info
	return nil
}

func (s *Service) UnregisterClient(id string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.clients, id)
}

func (s *Service) ClientCount() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return len(s.clients)
}

func (s *Service) ProcessAll(ctx context.Context) error {
	s.mu.RLock()
	ids := make([]string, 0, len(s.clients))
	for id := range s.clients {
		ids = append(ids, id)
	}
	s.mu.RUnlock()

	for _, id := range ids {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}
		s.mu.RLock()
		info := s.clients[id]
		s.mu.RUnlock()
		_ = info
	}
	return nil
}

// ── 匿名函数和闭包 ──────────────────────────────────

func Fn0049(base int) func(int) int {
	return func(x int) int {
		return base + x
	}
}

func Fn0050(increment int) func(...int) []int {
	return func(values ...int) []int {
		result := make([]int, len(values))
		for i, v := range values {
			result[i] = v + increment
		}
		return result
	}
}

func Fn0051(init func() int) func() int {
	var once sync.Once
	var value int
	return func() int {
		once.Do(func() {
			value = init()
		})
		return value
	}
}

// ── 带复杂字符串和注释的函数（测试括号匹配） ────────

func Fn0052() string {
	// 大括号在注释中: { 这不应该被匹配 } 结束
	return "这个字符串里面有大括号: { count: 3 } 以及更多"
}

func Fn0053() string {
	/* 块注释里面有 { 花括号 } 和字符串 "里面也有 { brace }" */
	return `原始字符串里面有 { 括号和 "引号" 混合 }`
}

func Fn0054() string {
	s1 := "{\"nested\": {\"level\": 3, \"items\": [1,2,3]}, \"status\": \"ok\"}"
	s2 := `{"template": {"vars": ["${name}", "${value}"]}}`
	s3 := "func(a, b int) { return a + b }"
	return s1 + s2 + s3
}

// ── 包含 R" 的 Go 代码（测试 matchBrace 不会被 C++ 原始字符串污染） ──

func Fn0055() string {
	// Go 中没有 R" 语法，但字符串内容可能恰好包含 R"
	// matchBrace 不应错误地跳过代码
	line := "检查 Result := R\"something\" 这种模式"
	return line
}

func Fn0056() string {
	rest := "string with R\"delim(content)delim\" inside"
	return rest
}

func Fn0057() string {
	a := "R\"("
	b := ")"
	c := ")delim\""
	_ = a
	_ = b
	_ = c
	return "this should NOT trigger CPP raw string skip"
}

// ── 大文件中的批量函数（200 个链式调用） ───────────

func Fn0101() int { return 1 }
func Fn0102() int { return Fn0101() + 2 }
func Fn0103() int { return Fn0102() + 3 }
func Fn0104() int { return Fn0103() + 4 }
func Fn0105() int { return Fn0104() + 5 }
func Fn0106() int { return Fn0105() + 6 }
func Fn0107() int { return Fn0106() + 7 }
func Fn0108() int { return Fn0107() + 8 }
func Fn0109() int { return Fn0108() + 9 }
func Fn0110() int { return Fn0109() + 10 }
func Fn0111() int { return Fn0110() + 11 }
func Fn0112() int { return Fn0111() + 12 }
func Fn0113() int { return Fn0112() + 13 }
func Fn0114() int { return Fn0113() + 14 }
func Fn0115() int { return Fn0114() + 15 }
func Fn0116() int { return Fn0115() + 16 }
func Fn0117() int { return Fn0116() + 17 }
func Fn0118() int { return Fn0117() + 18 }
func Fn0119() int { return Fn0118() + 19 }
func Fn0120() int { return Fn0119() + 20 }
func Fn0121() int { return Fn0120() + Fn0101() }
func Fn0122() int { return Fn0121() + Fn0102() }
func Fn0123() int { return Fn0122() + Fn0103() }
func Fn0124() int { return Fn0123() + Fn0104() }
func Fn0125() int { return Fn0124() + Fn0105() }
func Fn0126() int { return Fn0125() + Fn0106() }
func Fn0127() int { return Fn0126() + Fn0107() }
func Fn0128() int { return Fn0127() + Fn0108() }
func Fn0129() int { return Fn0128() + Fn0109() }
func Fn0130() int { return Fn0129() + Fn0110() }

func Fn0131() int { return Fn0130() * Fn0101() / Fn0102() }
func Fn0132() int { return Fn0131() - Fn0103() + Fn0104() }
func Fn0133() int { return Fn0132() * Fn0105() - Fn0106() }
func Fn0134() int { return Fn0133() + Fn0107() * Fn0108() }
func Fn0135() int { return Fn0134() - Fn0109() / Fn0110() }
func Fn0136() int { return Fn0135() ^ Fn0111() }
func Fn0137() int { return Fn0136() | Fn0112() }
func Fn0138() int { return Fn0137() & Fn0113() }
func Fn0139() int { return Fn0138() << 1 }
func Fn0140() int { return Fn0139() >> 1 }

// ── 接口和接口实现 ──────────────────────────────────

type Processor[T any] interface {
	Process(ctx context.Context, input T) (T, error)
	Validate(input T) bool
	Name() string
}

type BaseProcessor struct {
	name string
}

func NewBaseProcessor(name string) *BaseProcessor {
	return &BaseProcessor{name: name}
}

func (bp *BaseProcessor) Name() string { return bp.name }

type IntProcessor struct {
	BaseProcessor
	factor int
}

func NewIntProcessor(name string, factor int) *IntProcessor {
	return &IntProcessor{BaseProcessor: BaseProcessor{name: name}, factor: factor}
}

func (ip *IntProcessor) Process(ctx context.Context, input int) (int, error) {
	if !ip.Validate(input) {
		return 0, fmt.Errorf("invalid input: %d", input)
	}
	select {
	case <-ctx.Done():
		return 0, ctx.Err()
	default:
	}
	return input * ip.factor, nil
}

func (ip *IntProcessor) Validate(input int) bool {
	return input > 0 && input < 1000000
}

type StringProcessor struct {
	BaseProcessor
	prefix string
}

func NewStringProcessor(name, prefix string) *StringProcessor {
	return &StringProcessor{BaseProcessor: BaseProcessor{name: name}, prefix: prefix}
}

func (sp *StringProcessor) Process(ctx context.Context, input string) (string, error) {
	if !sp.Validate(input) {
		return "", fmt.Errorf("invalid input: %q", input)
	}
	return sp.prefix + input, ctx.Err()
}

func (sp *StringProcessor) Validate(input string) bool {
	return len(input) > 0 && len(input) < 10000
}

// ── defer/panic/recover 测试 ─────────────────────────

func Fn0141(items []int) (result []int, err error) {
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("panic: %v", r)
			result = nil
		}
	}()
	result = make([]int, 0, len(items))
	for _, item := range items {
		if item < 0 {
			panic(fmt.Sprintf("negative: %d", item))
		}
		result = append(result, item)
	}
	return result, nil
}

func Fn0142(cleanup func()) {
	defer cleanup()
	defer func() {
		if r := recover(); r != nil {
			fmt.Printf("recovered: %v\n", r)
		}
	}()
}

// ── 嵌套 map/slice 操作 ─────────────────────────────

func Fn0143[T any](items [][]T) []T {
	result := make([]T, 0, len(items)*3)
	for _, row := range items {
		for _, item := range row {
			result = append(result, item)
		}
	}
	return result
}

type Tree[T any] struct {
	Value    T
	Children []*Tree[T]
}

func Fn0144[T any](t *Tree[T], fn func(T) T) *Tree[T] {
	if t == nil {
		return nil
	}
	result := &Tree[T]{Value: fn(t.Value)}
	for _, child := range t.Children {
		result.Children = append(result.Children, Fn0144(child, fn))
	}
	return result
}

func Fn0145(m map[string]map[string][]int) map[string]int {
	result := make(map[string]int, len(m))
	for k, v := range m {
		total := 0
		for _, vals := range v {
			for _, val := range vals {
				total += val
			}
		}
		result[k] = total
	}
	return result
}

// ── go routine + waitgroup ──────────────────────────

func Fn0146(ctx context.Context, items []int, workers int) ([]int, error) {
	if workers <= 0 {
		workers = 4
	}
	jobs := make(chan int, len(items))
	results := make(chan int, len(items))
	var wg sync.WaitGroup

	for w := 0; w < workers; w++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for item := range jobs {
				select {
				case <-ctx.Done():
					return
				default:
					results <- item * item
				}
			}
		}()
	}

	go func() {
		for _, item := range items {
			jobs <- item
		}
		close(jobs)
	}()

	go func() {
		wg.Wait()
		close(results)
	}()

	output := make([]int, 0, len(items))
	for r := range results {
		output = append(output, r)
	}
	return output, nil
}

// ── switch 类型断言 ─────────────────────────────────

func Fn0147(v interface{}) string {
	switch val := v.(type) {
	case nil:
		return "nil"
	case int:
		return fmt.Sprintf("int:%d", val)
	case int8:
		return fmt.Sprintf("int8:%d", val)
	case int16:
		return fmt.Sprintf("int16:%d", val)
	case int32:
		return fmt.Sprintf("int32:%d", val)
	case int64:
		return fmt.Sprintf("int64:%d", val)
	case uint:
		return fmt.Sprintf("uint:%d", val)
	case uint8:
		return fmt.Sprintf("uint8:%d", val)
	case uint16:
		return fmt.Sprintf("uint16:%d", val)
	case uint32:
		return fmt.Sprintf("uint32:%d", val)
	case uint64:
		return fmt.Sprintf("uint64:%d", val)
	case float32:
		return fmt.Sprintf("float32:%f", val)
	case float64:
		return fmt.Sprintf("float64:%f", val)
	case string:
		return fmt.Sprintf("string:%q", val)
	case bool:
		return fmt.Sprintf("bool:%v", val)
	case []byte:
		return fmt.Sprintf("bytes:%d", len(val))
	case error:
		return fmt.Sprintf("error:%s", val.Error())
	default:
		return fmt.Sprintf("unknown:%T", val)
	}
}

func Fn0148(values []interface{}) []string {
	result := make([]string, len(values))
	for i, v := range values {
		result[i] = Fn0147(v)
	}
	return result
}

// ── init 函数 ────────────────────────────────────────

func init() {
	fmt.Printf("ast_stress package loaded: %d functions available\n", 148)
}
