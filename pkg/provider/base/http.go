package base

import (
	"context"
	"fmt"
	"io"
	"net/http"
)

const providerRequestUserAgent = "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/114.0.0.0 Safari/537.36"

// DoGetRequest 用于发送供应商查询所需的通用 GET 请求，并返回响应体内容。
// 参数含义：ctx 为请求上下文；httpCli 为执行请求的 HTTP 客户端；requestURL 为完整请求地址。
// 返回值：成功时返回响应体字节切片；若建请求、发请求、状态码校验或读取响应失败则返回错误。
func DoGetRequest(ctx context.Context, httpCli *http.Client, requestURL string) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, requestURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create service info request: %w", err)
	}
	req.Header.Set("User-Agent", providerRequestUserAgent)

	resp, err := httpCli.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to get service info: %w", err)
	}
	defer resp.Body.Close()

	// 供应商接口约定只有 200 响应才视为成功，其他状态直接中断避免解析异常页面。
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("failed to get service info, status code: %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read service info body: %w", err)
	}

	return body, nil
}
