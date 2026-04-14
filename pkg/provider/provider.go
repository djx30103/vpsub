package provider

import (
	"context"
	"errors"

	"github.com/djx30103/vpsub/pkg/provider/bandwagonhost"
	"github.com/djx30103/vpsub/pkg/provider/base"
	"github.com/djx30103/vpsub/pkg/provider/passthrough"
	"github.com/djx30103/vpsub/pkg/provider/racknerd"
)

const (
	ProviderType_BandwagonHost = "bandwagonhost"
	ProviderType_Racknerd      = "racknerd"
	ProviderType_Passthrough   = "passthrough"
)

type Provider interface {
	GetServiceInfo(ctx context.Context) (*base.APIResponseInfo, error)
}

func IsValidProvider(providerType string) bool {
	switch providerType {
	case ProviderType_BandwagonHost:
	case ProviderType_Racknerd:
	case ProviderType_Passthrough:

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
	case ProviderType_Passthrough:
		return passthrough.New(info), nil
	default:
		return nil, errors.New("unknown provider type")
	}
}
