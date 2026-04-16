package pathutil

import (
	"errors"
	"fmt"
	"strings"
)

var (
	ErrPathMustStartWithSlash = errors.New("path must start with /")
	errUnsafePath             = errors.New("path is unsafe")
	errUnsafeFile             = errors.New("file is unsafe")
)

// NormalizeRequestPath 用于统一归一化订阅请求路径，仅清理首尾空白并移除末尾连续斜杠。
// 参数含义：path 为原始请求路径或配置中的路由路径。
// 返回值：返回归一化后的路径；空串和单个根路径会原样保留。
func NormalizeRequestPath(path string) string {
	path = strings.TrimSpace(path)
	if path == "" || path == "/" {
		return path
	}

	return strings.TrimRight(path, "/")
}

// NormalizeRoutePath 用于统一归一化对外路由路径，避免出现重复或非法表示。
// 参数含义：path 为配置中的原始路由路径。
// 返回值：返回归一化后的路由路径和错误。
func NormalizeRoutePath(path string) (string, error) {
	path = NormalizeRequestPath(path)
	if path == "" {
		return "", fmt.Errorf("path is empty")
	}

	if !strings.HasPrefix(path, "/") {
		return "", ErrPathMustStartWithSlash
	}

	if !IsSafeRelativePath(strings.TrimPrefix(path, "/")) {
		return "", errUnsafePath
	}

	return path, nil
}

// NormalizeSubscriptionFilePath 用于统一归一化订阅文件相对路径，确保其始终位于订阅目录内。
// 参数含义：filePath 为配置中的订阅文件相对路径。
// 返回值：返回归一化后的文件相对路径和错误。
func NormalizeSubscriptionFilePath(filePath string) (string, error) {
	filePath = strings.TrimSpace(filePath)
	filePath = strings.ReplaceAll(filePath, "\\", "/")
	filePath = strings.TrimPrefix(filePath, "./")

	if !IsSafeRelativePath(filePath) {
		return "", errUnsafeFile
	}

	return filePath, nil
}

// IsSafeRelativePath 用于校验相对路径是否仍处于目标目录内，避免绝对路径、当前路径段、空路径段和上级目录越界。
// 参数含义：path 为待校验的相对路径。
// 返回值：返回该路径是否安全。
func IsSafeRelativePath(path string) bool {
	path = strings.TrimSpace(path)
	if path == "" {
		return false
	}

	if strings.HasPrefix(path, "/") || strings.HasPrefix(path, "\\") {
		return false
	}

	cleanPath := strings.ReplaceAll(path, "\\", "/")
	for _, segment := range strings.Split(cleanPath, "/") {
		if segment == "." || segment == ".." || segment == "" {
			return false
		}
	}

	return true
}
