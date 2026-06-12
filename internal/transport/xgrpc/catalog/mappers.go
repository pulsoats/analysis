package catalog

import (
	catalogpb "github.com/pulsoats/contracts/gen/go/catalog/v1"
	corepb "github.com/pulsoats/contracts/gen/go/core/v1"
	"github.com/pulsoats/core/detect"
	"github.com/pulsoats/core/exchange"
)

func mapToDetectorMetaPb(meta detect.DetectorMeta) *corepb.DetectorMeta {
	return &corepb.DetectorMeta{
		Code:        meta.Code,
		Version:     meta.Version,
		Kind:        string(meta.Kind),
		Description: meta.Description,
		OptsSchema:  meta.OptsSchema,
	}
}

func mapToListAvailableDetectorsPb(metas []detect.DetectorMeta) *catalogpb.ListAvailableDetectorsResponse {
	res := make([]*corepb.DetectorMeta, 0, len(metas))
	for _, m := range metas {
		res = append(res, mapToDetectorMetaPb(m))
	}
	return &catalogpb.ListAvailableDetectorsResponse{Detectors: res}
}

func mapToExchangeMetaPb(meta exchange.Meta) *corepb.ExchangeMeta {
	return &corepb.ExchangeMeta{
		Code:       meta.Code,
		Intervals:  meta.Intervals,
		Categories: meta.Categories,
	}
}

func mapToListAvailableExchangesPb(metas []exchange.Meta) *catalogpb.ListAvailableExchangesResponse {
	res := make([]*corepb.ExchangeMeta, 0, len(metas))
	for _, m := range metas {
		res = append(res, mapToExchangeMetaPb(m))
	}
	return &catalogpb.ListAvailableExchangesResponse{ExchangeMetas: res}
}
