package bytesize

import "fmt"

const (
	B  = 1
	KB = 1024 * B
	MB = 1024 * KB
	GB = 1024 * MB
	TB = 1024 * GB
)

type Unit string

const (
	UnitB Unit = "B"
	UnitK Unit = "K"
	UnitM Unit = "M"
	UnitG Unit = "G"
	UnitT Unit = "T"
)

func IsValidUnit(unit string) bool {
	return unit == string(UnitB) || unit == string(UnitK) ||
		unit == string(UnitM) || unit == string(UnitG) ||
		unit == string(UnitT)
}

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
		return B
	}
}

func Format(bytes int64, unit string) string {
	return fmt.Sprintf("%d%s", bytes/GetDivisor(unit), unit)
}
