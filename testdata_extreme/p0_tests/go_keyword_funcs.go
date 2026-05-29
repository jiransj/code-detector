package main

// Go: function names that are also keywords
// In Go, 'init' is a special function, NOT a keyword
// Other keywords CANNOT be used as function names in Go

func init() {
	// This is Go's special init function — valid
}

func main() {
	// This is Go's main function — valid
}

// These would be compile errors in Go:
// func if() {}     // 'if' is a keyword
// func for() {}    // 'for' is a keyword
// func return() {} // 'return' is a keyword

// But these built-in function names can be shadowed:
func make(t string, args ...int) []int {
	return nil
}

func new(x int) *int {
	return &x
}

func print(s string) {
	// shadows built-in print
}

func len(s string) int {
	return len(s)
}
