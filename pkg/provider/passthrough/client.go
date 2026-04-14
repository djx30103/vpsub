package passthrough

import (
	"context"

	"github.com/djx30103/vpsub/pkg/provider/base"
)

type Client struct{}

// New 用于创建 passthrough 客户端。
// passthrough 不调用任何外部 API，订阅文件原样返回。
func New(_ base.APIRequestInfo) *Client {
	return &Client{}
}

// GetServiceInfo 直接返回 nil，不请求任何服务商接口。
func (c *Client) GetServiceInfo(_ context.Context) (*base.APIResponseInfo, error) {
	return nil, nil
}
