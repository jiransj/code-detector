package ast_stress

// ────────────────────────────────────────────────────────────
// Go 泛型复杂场景压力测试
// 测试 tree-sitter 泛型解析的边界情况：
// 多类型参数、嵌套泛型、约束接口、泛型方法等
// ────────────────────────────────────────────────────────────

import (
	"context"
	"fmt"
)

// ── 泛型约束接口 ───────────────────────────────────

type Number interface {
	~int | ~int8 | ~int16 | ~int32 | ~int64 |
		~uint | ~uint8 | ~uint16 | ~uint32 | ~uint64 |
		~float32 | ~float64
}

type ComparableHasher interface {
	comparable
	Hash() uint64
}

type StringConstraint interface {
	~string
}

// ── 泛型结构体 ─────────────────────────────────────

type Box[T any] struct {
	Value T
}

type Pair[T, U any] struct {
	First  T
	Second U
}

type Triple[T, U, V any] struct {
	A T
	B U
	C V
}

type Node[T ComparableHasher] struct {
	Value    T
	Children []*Node[T]
}

// ── 泛型函数 ───────────────────────────────────────

// Generic1 单类型参数泛型函数
func Generic1[T any](items []T) []T {
	result := make([]T, 0, len(items))
	result = append(result, items...)
	return result
}

// Generic2 双类型参数泛型函数
func Generic2[K comparable, V any](m map[K]V, keys []K) []V {
	result := make([]V, 0, len(keys))
	for _, k := range keys {
		if v, ok := m[k]; ok {
			result = append(result, v)
		}
	}
	return result
}

// Generic3 三类型参数泛型函数
func Generic3[A, B, C any](a A, b B, c C) Triple[A, B, C] {
	return Triple[A, B, C]{A: a, B: b, C: c}
}

// Generic4 带约束的泛型函数
func Generic4[T Number](a, b T) T {
	if a > b {
		return a
	}
	return b
}

// Generic5 泛型 slice 聚合
func Generic5[T Number](values []T) (sum T, avg float64) {
	if len(values) == 0 {
		return 0, 0
	}
	for _, v := range values {
		sum += v
	}
	return sum, float64(sum) / float64(len(values))
}

// Generic6 泛型 map/reduce 操作
func Generic6[K comparable, V any](items []K, fn func(K) V) map[K]V {
	result := make(map[K]V, len(items))
	for _, item := range items {
		result[item] = fn(item)
	}
	return result
}

// Generic7 泛型方法接收器
func (b *Box[T]) Set(val T) {
	b.Value = val
}

func (b *Box[T]) Get() T {
	return b.Value
}

// Generic8 泛型方法调用
func Generic8[T any](b *Box[T], val T) T {
	b.Set(val)
	return b.Get()
}

// Generic9 嵌套泛型
func Generic9[T any](items []Box[T]) []T {
	result := make([]T, len(items))
	for i, b := range items {
		result[i] = b.Get()
	}
	return result
}

// Generic10 泛型 + context + error 组合
func Generic10[T any](ctx context.Context, items []T, fn func(context.Context, T) (T, error)) ([]T, error) {
	result := make([]T, 0, len(items))
	for _, item := range items {
		select {
		case <-ctx.Done():
			return result, ctx.Err()
		default:
		}
		processed, err := fn(ctx, item)
		if err != nil {
			return result, fmt.Errorf("process item: %w", err)
		}
		result = append(result, processed)
	}
	return result, nil
}

// Generic11 泛型 String() 方法
func (p Pair[T, U]) String() string {
	return fmt.Sprintf("(%v, %v)", p.First, p.Second)
}

// Generic12 泛型递归（树遍历）
func Generic12[T ComparableHasher](node *Node[T], target T) *Node[T] {
	if node == nil {
		return nil
	}
	if node.Value == target {
		return node
	}
	for _, child := range node.Children {
		if found := Generic12(child, target); found != nil {
			return found
		}
	}
	return nil
}

// Generic13 类型参数推导调用
func Generic13() {
	b := Box[int]{Value: 42}
	_ = Generic8[int](&b, 100)
	_ = Generic4(3.14, 2.718)
	_ = Generic5([]int{1, 2, 3, 4, 5})
	_ = Generic2(map[string]int{"a": 1}, []string{"a"})
}
