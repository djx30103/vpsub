package main

import (
	"bytes"
	"testing"
)

// TestPrintConfigPath_WritesReadableLine 用于验证启动阶段打印配置路径时会输出到指定 writer，且格式完整可读。
// 参数含义：t 为测试上下文。
// 返回值：无。
func TestPrintConfigPath_WritesReadableLine(t *testing.T) {
	t.Parallel()

	var buffer bytes.Buffer
	printConfigPath(&buffer, "/tmp/config.yml")

	if got, want := buffer.String(), "using config path: /tmp/config.yml\n"; got != want {
		t.Fatalf("expected %q, got %q", want, got)
	}
}
