package provider

import (
	"context"

	"vpsub/pkg/provider/bandwagonhost"
	"vpsub/pkg/provider/base"
	"vpsub/pkg/provider/racknerd"

	"github.com/pkg/errors"
)

const (
	ProviderType_BandwagonHost = "bandwagonhost"
	ProviderType_Racknerd      = "racknerd"
)

type Provider interface {
	GetServiceInfo(ctx context.Context) (*base.APIResponseInfo, error)
}

func IsValidProvider(providerType string) bool {
	switch providerType {
	case ProviderType_BandwagonHost:
	case ProviderType_Racknerd:

	default:
		return false
	}

	return true
}

func NewProvider(info base.APIRequestInfo) (Provider, error) {
	switch info.ProviderType {
	case ProviderType_BandwagonHost:
		return bandwagonhost.New(info), nil
	case ProviderType_Racknerd:
		return racknerd.New(info), nil
	default:
		return nil, errors.New("unknown provider type")
	}
}
