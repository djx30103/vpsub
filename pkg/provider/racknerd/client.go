package racknerd

import (
	"context"
	"encoding/csv"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
	_ "time/tzdata"

	"github.com/djx30103/vpsub/pkg/provider/base"
)

var rackNerdResetLocation = func() *time.Location {
	loc, err := time.LoadLocation("America/Los_Angeles")
	if err != nil {
		panic(fmt.Errorf("load racknerd reset timezone: %w", err))
	}
	return loc
}()

type Client struct {
	apiKey  string
	apiHash string

	baseURL string
	httpCli *http.Client
}

// New 用于根据配置创建 RackNerd API 客户端。
// 参数含义：info 为调用接口所需的认证信息和请求超时配置。
// 返回值：返回初始化完成的 RackNerd 客户端。
func New(info base.APIRequestInfo) *Client {
	return &Client{
		apiHash: info.APIID,
		apiKey:  info.APIKey,
		baseURL: "https://nerdvm.racknerd.com",
		httpCli: &http.Client{
			Timeout: info.RequestTimeout,
		},
	}
}

// GetServiceInfo 用于查询 RackNerd 当前服务的流量使用情况。
// 参数含义：ctx 为本次请求的上下文，用于控制超时和取消。
// 返回值：返回统一格式的流量信息；若请求或解析失败则返回错误。
func (c *Client) GetServiceInfo(ctx context.Context) (*base.APIResponseInfo, error) {
	reqURL, err := url.Parse(c.baseURL + "/api/client/command.php")
	if err != nil {
		return nil, fmt.Errorf("failed to parse service info url: %w", err)
	}
	query := reqURL.Query()
	query.Set("key", c.apiKey)
	query.Set("hash", c.apiHash)
	query.Set("action", "info")
	query.Set("bw", "true")
	reqURL.RawQuery = query.Encode()

	body, err := base.DoGetRequest(ctx, c.httpCli, reqURL.String())
	if err != nil {
		return nil, err
	}

	info, err := parseServiceInfoResponse(string(body))
	if err != nil {
		return nil, err
	}

	// RackNerd 流量每月 1 日固定重置（太平洋时区），API 不返回该字段，此处直接硬编码。
	// 来源：https://lowendtalk.com/discussion/185395/racknerd-vps-bandwidth
	now := time.Now()
	info.Expire = nextResetUnix(now, 1, rackNerdResetLocation)

	return info, nil
}

// nextResetUnix 用于按指定时区计算每月重置日对应的下一次重置时间戳。
// 参数含义：now 为当前时间；resetDay 为每月重置日；loc 为重置规则所依据的时区。
// 返回值：返回下一次重置时间的 Unix 时间戳。
func nextResetUnix(now time.Time, resetDay int, loc *time.Location) int64 {
	current := now.In(loc)
	thisMonth := time.Date(current.Year(), current.Month(), resetDay, 0, 0, 0, 0, loc)
	if current.Before(thisMonth) {
		return thisMonth.Unix()
	}
	return time.Date(current.Year(), current.Month()+1, resetDay, 0, 0, 0, 0, loc).Unix()
}

// parseServiceInfoResponse 用于解析 RackNerd 返回的原始流量响应。
// 参数含义：raw 为 RackNerd 接口返回的原始文本。
// 返回值：返回统一格式的流量信息；若字段缺失、格式非法或数值异常则返回错误。
func parseServiceInfoResponse(raw string) (*base.APIResponseInfo, error) {
	// 上游返回的是类 CSV 文本，且部分场景下数值字段会被双引号包裹，这里统一按 CSV 解析以兼容两种格式。
	reader := csv.NewReader(strings.NewReader(raw))
	reader.FieldsPerRecord = -1
	reader.TrimLeadingSpace = true

	record, err := reader.Read()
	if err != nil && !errors.Is(err, io.EOF) {
		return nil, fmt.Errorf("failed to parse service info csv: %w", err)
	}

	// 业务上至少需要 total 和 used 两个字段，缺任意一个都无法生成订阅剩余流量信息。
	if len(record) < 2 {
		return nil, fmt.Errorf("failed to parse service info, invalid return response: %s", raw)
	}

	total, err := strconv.ParseInt(record[0], 10, 64)
	if err != nil {
		return nil, fmt.Errorf("failed to parse total: %w", err)
	}

	used, err := strconv.ParseInt(record[1], 10, 64)
	if err != nil {
		return nil, fmt.Errorf("failed to parse used: %w", err)
	}

	// 上游返回 0 通常代表接口异常或服务信息不可用，这里直接视为错误避免下游展示错误配额。
	if total <= 0 || used < 0 {
		return nil, errors.New("failed to get service info, total or used is 0")
	}

	// RackNerd API 只返回总用量，不区分上传和下载，因此各取一半作为近似值。
	return &base.APIResponseInfo{
		Upload:   used / 2,
		Download: used / 2,
		Total:    total,
		Expire:   0,
	}, nil
}
