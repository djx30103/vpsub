package config

import (
	"strings"
	"testing"
)

// TestUsageDisplayConfig_RejectsResetTimeTemplateWithHour 用于验证重置时间模板不允许引用时分秒占位符。
// 参数含义：t 为测试上下文。
// 返回值：无。
func TestUsageDisplayConfig_RejectsResetTimeTemplateWithHour(t *testing.T) {
	t.Parallel()

	conf := &UsageDisplayConfig{
		TrafficFormat:   "已用 {{.used}} / {{.total}}",
		TrafficUnit:     "G",
		ResetTimeFormat: "重置 {{.year}}-{{.month}}-{{.day}} {{.hour}}",
	}

	err := conf.validate()
	if err == nil || !strings.Contains(err.Error(), "reset_time_format") {
		t.Fatalf("expected reset_time_format validation error, got: %v", err)
	}
}
