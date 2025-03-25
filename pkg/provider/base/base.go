package base

import "time"

type APIResponseInfo struct {
	Upload   int64
	Download int64
	Total    int64
	Expire   int64
}

type APIRequestInfo struct {
	APIID          string
	APIKey         string
	ProviderType   string
	RequestTimeout time.Duration
}
