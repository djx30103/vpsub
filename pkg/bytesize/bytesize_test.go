package bytesize

import "testing"

// TestIsValidUnit_拒绝字节单位 用于验证取消 B 单位后，B 不再被视为合法流量显示单位。
// 参数含义：t 为测试上下文，用于驱动断言和失败输出。
// 返回值：无，测试失败时由 testing 框架终止当前用例。
func TestIsValidUnit_拒绝字节单位(t *testing.T) {
	if IsValidUnit("B") {
		t.Fatalf("expected B to be invalid")
	}
}

// TestGetDivisor_未知单位回退字节基数 用于验证未知单位会回退到 1，避免格式化逻辑被非法单位放大或缩小。
// 参数含义：t 为测试上下文，用于驱动断言和失败输出。
// 返回值：无，测试失败时由 testing 框架终止当前用例。
func TestGetDivisor_未知单位回退字节基数(t *testing.T) {
	if got := GetDivisor("B"); got != 1 {
		t.Fatalf("expected divisor for B to be 1, got %d", got)
	}

	if got := GetDivisor(""); got != 1 {
		t.Fatalf("expected divisor for empty unit to be 1, got %d", got)
	}
}

// TestConstants_使用二进制位移定义单位 用于验证 K 到 T 的单位常量与二进制换算结果一致。
// 参数含义：t 为测试上下文，用于驱动断言和失败输出。
// 返回值：无，测试失败时由 testing 框架终止当前用例。
func TestConstants_使用二进制位移定义单位(t *testing.T) {
	if KB != 1<<10 {
		t.Fatalf("expected KB to equal 1<<10, got %d", KB)
	}

	if MB != 1<<20 {
		t.Fatalf("expected MB to equal 1<<20, got %d", MB)
	}

	if GB != 1<<30 {
		t.Fatalf("expected GB to equal 1<<30, got %d", GB)
	}

	if TB != 1<<40 {
		t.Fatalf("expected TB to equal 1<<40, got %d", TB)
	}
}
