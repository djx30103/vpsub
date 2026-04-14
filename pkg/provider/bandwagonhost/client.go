package bandwagonhost

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"

	"github.com/djx30103/vpsub/pkg/provider/base"
)

type Client struct {
	veid   string
	apiKey string

	baseURL string
	httpCli *http.Client
}

// New 用于根据账号信息创建 BandwagonHost API 客户端。
// 参数含义：info 为调用接口所需的认证信息和请求超时配置。
// 返回值：返回初始化完成的客户端。
func New(info base.APIRequestInfo) *Client {
	return &Client{
		veid:    info.APIID,
		apiKey:  info.APIKey,
		baseURL: "https://api.64clouds.com",
		httpCli: &http.Client{
			Timeout: info.RequestTimeout,
		},
	}
}

// GetServiceInfo 用于查询 BandwagonHost 当前服务的流量使用情况。
// 参数含义：ctx 为本次请求的上下文，用于控制超时和取消。
// 返回值：返回统一格式的流量信息；若请求或解析失败则返回错误。
func (c *Client) GetServiceInfo(ctx context.Context) (*base.APIResponseInfo, error) {
	reqURL, err := url.Parse(c.baseURL + "/v1/getServiceInfo")
	if err != nil {
		return nil, fmt.Errorf("failed to parse service info url: %w", err)
	}
	query := reqURL.Query()
	query.Set("veid", c.veid)
	query.Set("api_key", c.apiKey)
	reqURL.RawQuery = query.Encode()

	body, err := base.DoGetRequest(ctx, c.httpCli, reqURL.String())
	if err != nil {
		return nil, err
	}

	info := new(ServiceInfo)
	if err := json.Unmarshal(body, info); err != nil {
		return nil, fmt.Errorf("failed to unmarshal service info: %w", err)
	}

	// BandwagonHost API 只返回总用量，不区分上传和下载，因此各取一半作为近似值。
	half := info.DataCounter * info.MonthlyDataMultiplier / 2
	total := info.PlanMonthlyData * info.MonthlyDataMultiplier

	// 总量为 0 代表套餐信息异常；已用流量为 0 则是正常场景，例如新开机或刚重置流量。
	if total <= 0 {
		return nil, errors.New("failed to get service info, total is 0")
	}

	return &base.APIResponseInfo{
		Upload:   half,
		Download: half,
		Total:    total,
		Expire:   info.DataNextReset,
	}, nil
}
