package pathutil

import (
	"errors"
	"testing"
)

// TestNormalizeRequestPath_TrimsTrailingSlashes 用于验证请求路径归一化时会删除末尾连续斜杠，同时保留中间路径结构。
// 参数含义：t 为测试上下文。
// 返回值：无。
func TestNormalizeRequestPath_TrimsTrailingSlashes(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name string
		path string
		want string
	}{
		{
			name: "single trailing slash",
			path: "/test.yaml/",
			want: "/test.yaml",
		},
		{
			name: "multiple trailing slashes",
			path: "/test.yaml//",
			want: "/test.yaml",
		},
		{
			name: "nested path single trailing slash",
			path: "/a/b/c/",
			want: "/a/b/c",
		},
		{
			name: "nested path multiple trailing slashes",
			path: "/a/b/c///",
			want: "/a/b/c",
		},
		{
			name: "only trailing slashes remain empty",
			path: "//",
			want: "",
		},
		{
			name: "middle slash preserved",
			path: "/group//node///",
			want: "/group//node",
		},
	}

	for _, testCase := range testCases {
		testCase := testCase

		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			got := NormalizeRequestPath(testCase.path)
			if got != testCase.want {
				t.Fatalf("NormalizeRequestPath(%q) = %q, want %q", testCase.path, got, testCase.want)
			}
		})
	}
}

// TestNormalizeRoutePath 用于验证对外路由路径会统一归一化，并拒绝非法或越界路径。
// 参数含义：t 为测试上下文。
// 返回值：无。
func TestNormalizeRoutePath(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name    string
		path    string
		want    string
		wantErr bool
	}{
		{
			name: "trim spaces and trailing slashes",
			path: " /group/a.yaml// ",
			want: "/group/a.yaml",
		},
		{
			name:    "reject empty path",
			path:    "   ",
			wantErr: true,
		},
		{
			name:    "reject path without leading slash",
			path:    "group/a.yaml",
			wantErr: true,
		},
		{
			name:    "reject parent segment",
			path:    "/group/../a.yaml",
			wantErr: true,
		},
		{
			name:    "reject empty middle segment",
			path:    "/group//a.yaml",
			wantErr: true,
		},
		{
			name:    "reject dot segment",
			path:    "/group/./a.yaml",
			wantErr: true,
		},
	}

	for _, testCase := range testCases {
		testCase := testCase

		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			got, err := NormalizeRoutePath(testCase.path)
			if testCase.wantErr {
				if err == nil {
					t.Fatalf("NormalizeRoutePath(%q) expected error", testCase.path)
				}
				if testCase.path == "group/a.yaml" && !errors.Is(err, ErrPathMustStartWithSlash) {
					t.Fatalf("NormalizeRoutePath(%q) expected ErrPathMustStartWithSlash, got %v", testCase.path, err)
				}
				return
			}

			if err != nil {
				t.Fatalf("NormalizeRoutePath(%q) returned error: %v", testCase.path, err)
			}

			if got != testCase.want {
				t.Fatalf("NormalizeRoutePath(%q) = %q, want %q", testCase.path, got, testCase.want)
			}
		})
	}
}

// TestNormalizeSubscriptionFilePath 用于验证订阅文件相对路径会统一归一化，并拒绝越界写法。
// 参数含义：t 为测试上下文。
// 返回值：无。
func TestNormalizeSubscriptionFilePath(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name    string
		path    string
		want    string
		wantErr bool
	}{
		{
			name: "trim dot prefix and windows separator",
			path: " ./group\\a.yaml ",
			want: "group/a.yaml",
		},
		{
			name:    "reject absolute path",
			path:    "/tmp/a.yaml",
			wantErr: true,
		},
		{
			name:    "reject parent segment",
			path:    "../a.yaml",
			wantErr: true,
		},
		{
			name:    "reject empty path",
			path:    " ",
			wantErr: true,
		},
		{
			name:    "reject dot segment",
			path:    "dir/./x.yaml",
			wantErr: true,
		},
	}

	for _, testCase := range testCases {
		testCase := testCase

		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			got, err := NormalizeSubscriptionFilePath(testCase.path)
			if testCase.wantErr {
				if err == nil {
					t.Fatalf("NormalizeSubscriptionFilePath(%q) expected error", testCase.path)
				}
				return
			}

			if err != nil {
				t.Fatalf("NormalizeSubscriptionFilePath(%q) returned error: %v", testCase.path, err)
			}

			if got != testCase.want {
				t.Fatalf("NormalizeSubscriptionFilePath(%q) = %q, want %q", testCase.path, got, testCase.want)
			}
		})
	}
}

// TestSafePathHelpers 用于验证相对路径安全辅助方法会拒绝越界输入。
// 参数含义：t 为测试上下文。
// 返回值：无。
func TestSafePathHelpers(t *testing.T) {
	t.Parallel()

	if !IsSafeRelativePath("group/a.yaml") {
		t.Fatalf("expected relative subscription path to be safe")
	}

	if IsSafeRelativePath("../a.yaml") {
		t.Fatalf("expected parent segment to be unsafe")
	}

	if IsSafeRelativePath("dir/./x.yaml") {
		t.Fatalf("expected dot segment to be unsafe")
	}
}
