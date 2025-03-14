package bandwagonhost

import (
	"context"

	"vpsub/pkg/config"
	"vpsub/pkg/provider"

	"github.com/imroc/req/v3"
)

type Client struct {
	veid   string
	apikey string

	httpCli *req.Client
}

func New(pathConf config.PathConfig) *Client {
	return &Client{
		veid:   pathConf.APIID,
		apikey: pathConf.APIKey,
		httpCli: req.C().
			SetTimeout(*pathConf.Provider.RequestTimeout).
			SetUserAgent("Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/114.0.0.0 Safari/537.36"),
	}
}

func (c *Client) GetServiceInfo(ctx context.Context) (*provider.APIResponseInfo, error) {
	resp, err := c.httpCli.R().SetContext(ctx).
		SetQueryParam("veid", c.veid).
		SetQueryParam("api_key", c.apikey).
		Get("https://api.64clouds.com/v1/getServiceInfo")
	if err != nil {
		return nil, err
	}
	info := new(ServiceInfo)
	err = resp.UnmarshalJson(info)
	if err != nil {
		return nil, err
	}

	half := info.DataCounter * info.MonthlyDataMultiplier / 2
	total := info.PlanMonthlyData * info.MonthlyDataMultiplier

	return &provider.APIResponseInfo{
		Upload:   half,
		Download: half,
		Total:    total,
		Expire:   info.DataNextReset,
	}, nil
}
