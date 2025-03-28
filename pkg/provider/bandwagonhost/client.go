package bandwagonhost

import (
	"context"
	"net/http"

	"vpsub/pkg/provider/base"

	"github.com/imroc/req/v3"
	"github.com/pkg/errors"
)

type Client struct {
	veid   string
	apiKey string

	httpCli *req.Client
}

func New(info base.APIRequestInfo) *Client {
	return &Client{
		veid:   info.APIID,
		apiKey: info.APIKey,
		httpCli: req.C().
			SetTimeout(info.RequestTimeout).
			SetUserAgent("Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/114.0.0.0 Safari/537.36"),
	}
}

func (c *Client) GetServiceInfo(ctx context.Context) (*base.APIResponseInfo, error) {
	resp, err := c.httpCli.R().SetContext(ctx).
		SetQueryParam("veid", c.veid).
		SetQueryParam("api_key", c.apiKey).
		Get("https://api.64clouds.com/v1/getServiceInfo")
	if err != nil {
		return nil, errors.Wrap(err, "failed to get service info")
	}

	if resp.StatusCode != http.StatusOK {
		return nil, errors.Errorf("failed to get service info, status code: %d", resp.StatusCode)
	}

	info := new(ServiceInfo)
	err = resp.UnmarshalJson(info)
	if err != nil {
		return nil, errors.Wrap(err, "failed to unmarshal service info")
	}

	half := info.DataCounter * info.MonthlyDataMultiplier / 2
	total := info.PlanMonthlyData * info.MonthlyDataMultiplier

	if total == 0 || half == 0 {
		return nil, errors.New("failed to get service info, total or used is 0")
	}

	return &base.APIResponseInfo{
		Upload:   half,
		Download: half,
		Total:    total,
		Expire:   info.DataNextReset,
	}, nil
}
