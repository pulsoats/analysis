package catalog

import (
	catalogpb "github.com/pulsoats/contracts/gen/go/catalog/v1"
	corepb "github.com/pulsoats/contracts/gen/go/core/v1"
	"github.com/pulsoats/core/detect/detector"
	"github.com/pulsoats/core/detect/filter"
	"github.com/pulsoats/core/exchange"
)

func mapToDetectorMetaPb(meta detector.Meta) *corepb.DetectorMeta {
	return &corepb.DetectorMeta{
		Code:        meta.Code,
		Version:     meta.Version,
		Description: meta.Description,
		OptsSchema:  meta.OptsSchema,
	}
}

func mapToListAvailableDetectorsPb(metas []detector.Meta) *catalogpb.ListAvailableDetectorsResponse {
	res := make([]*corepb.DetectorMeta, 0, len(metas))
	for _, m := range metas {
		res = append(res, mapToDetectorMetaPb(m))
	}
	return &catalogpb.ListAvailableDetectorsResponse{Metas: res}
}

func mapToFilterMetaPb(meta filter.Meta) *corepb.FilterMeta {
	return &corepb.FilterMeta{
		Code:        meta.Code,
		Description: meta.Description,
	}
}

func mapToListAvailableFiltersPb(metas []filter.Meta) *catalogpb.ListAvailableFiltersResponse {
	res := make([]*corepb.FilterMeta, 0, len(metas))
	for _, m := range metas {
		res = append(res, mapToFilterMetaPb(m))
	}
	return &catalogpb.ListAvailableFiltersResponse{Metas: res}
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
	return &catalogpb.ListAvailableExchangesResponse{Metas: res}
}
