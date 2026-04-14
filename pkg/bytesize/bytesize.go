package bytesize

import "fmt"

const (
	_        = iota
	KB int64 = 1 << (10 * iota)
	MB
	GB
	TB
)

type Unit string

const (
	UnitK Unit = "K"
	UnitM Unit = "M"
	UnitG Unit = "G"
	UnitT Unit = "T"
)

// IsValidUnit 用于校验流量展示单位是否属于支持的范围。
// 参数含义：unit 为待校验的流量单位字符串。
// 返回值：当 unit 为 K、M、G、T 之一时返回 true，否则返回 false。
func IsValidUnit(unit string) bool {
	return unit == string(UnitK) || unit == string(UnitM) || unit == string(UnitG) ||
		unit == string(UnitT)
}

// GetDivisor 用于根据流量单位返回格式化时使用的除数。
// 参数含义：unit 为流量单位字符串。
// 返回值：返回对应单位的二进制除数，未知单位时回退为 1。
func GetDivisor(unit string) int64 {
	switch unit {
	case string(UnitT):
		return TB
	case string(UnitG):
		return GB
	case string(UnitM):
		return MB
	case string(UnitK):
		return KB
	default:
		return 1
	}
}

// Format 用于按指定流量单位格式化字节数展示值。
// 参数含义：bytes 为原始字节数，unit 为目标展示单位。
// 返回值：返回拼接单位后的整数字符串。
func Format(bytes int64, unit string) string {
	return fmt.Sprintf("%d%s", bytes/GetDivisor(unit), unit)
}
