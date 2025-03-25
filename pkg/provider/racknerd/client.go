package racknerd

import (
	"context"
	"net/http"
	"strconv"
	"strings"

	"vpsub/pkg/provider/base"

	"github.com/imroc/req/v3"
	"github.com/pkg/errors"
)

type Client struct {
	apiKey  string
	apiHash string

	httpCli *req.Client
}

func New(info base.APIRequestInfo) *Client {
	return &Client{
		apiHash: info.APIID,
		apiKey:  info.APIKey,
		httpCli: req.C().
			SetTimeout(info.RequestTimeout).
			SetUserAgent("Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/114.0.0.0 Safari/537.36"),
	}
}

func (c *Client) GetServiceInfo(ctx context.Context) (*base.APIResponseInfo, error) {
	resp, err := c.httpCli.R().SetContext(ctx).
		SetQueryParam("key", c.apiKey).
		SetQueryParam("hash", c.apiHash).
		SetQueryParam("action", "info").
		SetQueryParam("bw", "true").
		Get("https://nerdvm.racknerd.com/api/client/command.php")
	if err != nil {
		return nil, errors.Wrap(err, "failed to get service info")
	}

	if resp.StatusCode != http.StatusOK {
		return nil, errors.Errorf("failed to get service info, status code: %d", resp.StatusCode)
	}

	// total,used,free,percentused
	// 4294967296000,3212876925,4291754419075,0successracknerd-58c7b7xx.xx.xx.xx

	info := strings.Split(resp.String(), ",")
	if len(info) <= 3 {
		return nil, errors.Errorf("failed to parse service info, invalid return response: %s", resp.String())
	}

	total, err := strconv.ParseInt(info[0], 10, 64)
	if err != nil {
		return nil, errors.Wrap(err, "failed to parse total")
	}

	used, err := strconv.ParseInt(info[1], 10, 64)
	if err != nil {
		return nil, errors.Wrap(err, "failed to parse used")
	}

	if total <= 0 || used <= 0 {
		return nil, errors.New("failed to get service info, total or used is 0")
	}

	return &base.APIResponseInfo{
		Upload:   used / 2,
		Download: used / 2,
		Total:    total,
		Expire:   0,
	}, nil
}
