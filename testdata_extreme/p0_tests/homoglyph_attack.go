package main

import "fmt"

// --- Real Latin Hello ---
func Hello(name string) string {
	return "Latin: " + name
}

// --- Cyrillic Homoglyph "Hello" ---
// H is Cyrillic Н (U+041D), e is Cyrillic е (U+0435)
// These look IDENTICAL to Latin "Hello" but are different Unicode
func Неllо(name string) string {
	return "Cyrillic: " + name
}

// --- Latin "func" ---
func realFunc() string {
	return "real"
}

// --- Cyrillic Homoglyph "func" ---
// f is Latin f, u is Latin u, n is Latin n, c is Cyrillic с (U+0441)
// This function's keyword is `funс` not `func` — different Unicode
// Go compiler will reject this! But scanner might try to match it.

// --- Mexican standoff: three functions that all LOOK like processData() ---
func ProcessData(items []string) []string {     // Latin 'P', Latin 'D'
	return items
}

func РrосеssData(items []string) []string { // Р=Latin? No, Р=Cyrillic U+0420! 
	return items
}

func РrосеssDаtа(items []string) []string { // а=Cyrillic U+0430, not Latin a
	return items
}

// --- Latin "CallMe" ---
func CallMe() string {
	return "Latin CallMe"
}

// --- Cyrillic "CallMe" — C is Cyrillic С (U+0421) ---
func СallMе() string {
	return "Cyrillic CallMe"
}

func main() {
	fmt.Println(Hello("world"))
}
