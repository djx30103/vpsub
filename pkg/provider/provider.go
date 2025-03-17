package provider

import "context"

type Provider interface {
	GetServiceInfo(ctx context.Context) (*APIResponseInfo, error)
}
