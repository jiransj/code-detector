package parser

import (
	"sort"
	"strings"
)

// FileLines 预计算行偏移的文本结构，将 lineOffset/lineFromOffset 从 O(n) 降为 O(1)/O(log n)
type FileLines struct {
	text    string
	lines   []string // 延迟计算
	offsets []int    // offsets[i] = 第 i 行的字节偏移（含换行符位置）
}

// NewFileLines 从字符串构建行偏移表
func NewFileLines(text string) *FileLines {
	offsets := []int{0}
	for i := 0; i < len(text); i++ {
		if text[i] == '\n' {
			offsets = append(offsets, i+1)
		}
	}
	return &FileLines{text: text, offsets: offsets}
}

// NewFileLinesFromBytes 从 []byte 构建（延迟 string 转换，与旧接口兼容）
func NewFileLinesFromBytes(content []byte) *FileLines {
	return NewFileLines(string(content))
}

// LineOffset 返回第 lineIdx 行的字节偏移（O(1)）
func (fl *FileLines) LineOffset(lineIdx int) int {
	if lineIdx <= 0 {
		return 0
	}
	if lineIdx >= len(fl.offsets) {
		return len(fl.text)
	}
	return fl.offsets[lineIdx]
}

// LineFromOffset 根据字节偏移反推算行号（O(log n) 二分查找）
func (fl *FileLines) LineFromOffset(offset int) int {
	if offset <= 0 {
		return 0
	}
	idx := sort.Search(len(fl.offsets), func(i int) bool {
		return fl.offsets[i] > offset
	})
	return idx - 1
}

// Lines 返回行切片（延迟计算，仅首次调用时 Split）
func (fl *FileLines) Lines() []string {
	if fl.lines != nil {
		return fl.lines
	}
	fl.lines = strings.Split(fl.text, "\n")
	return fl.lines
}

// Text 返回完整文本
func (fl *FileLines) Text() string { return fl.text }

// NumLines 返回行数
func (fl *FileLines) NumLines() int { return len(fl.offsets) }

// LineContent 返回第 lineIdx 行的内容（不含换行符）
func (fl *FileLines) LineContent(lineIdx int) string {
	start := fl.LineOffset(lineIdx)
	end := fl.LineOffset(lineIdx + 1)
	if end > start && fl.text[end-1] == '\n' {
		end-- // 去掉末尾 \n
	}
	if start >= end {
		return ""
	}
	return fl.text[start:end]
}
