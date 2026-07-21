package catalog

import (
	"fmt"
	"sort"

	"github.com/pulsoats/core/detect/detector"
	"github.com/pulsoats/core/detect/filter"
	"github.com/pulsoats/core/exchange"
)

type Config struct {
	DetectorRegistry *detector.Registry
	FilterRegistry   *filter.Registry
	Exchanges        map[string]exchange.PublicClient
}

type Application struct {
	detectorRegistry *detector.Registry
	filterRegistry   *filter.Registry
	exchanges        map[string]exchange.PublicClient
}

func NewApplication(cfg Config) (*Application, error) {
	if cfg.DetectorRegistry == nil {
		return nil, fmt.Errorf("catalog app: detector registry is nil")
	}
	if cfg.FilterRegistry == nil {
		return nil, fmt.Errorf("catalog app: filter registry is nil")
	}
	if len(cfg.Exchanges) == 0 {
		return nil, fmt.Errorf("catalog app: empty exchanges map")
	}

	return &Application{
		detectorRegistry: cfg.DetectorRegistry,
		filterRegistry:   cfg.FilterRegistry,
		exchanges:        cfg.Exchanges,
	}, nil
}

func (a *Application) AvailableDetectors() []detector.Meta {
	return a.detectorRegistry.ListMetas()
}

func (a *Application) AvailableFilters() []filter.Meta {
	return a.filterRegistry.ListMetas()
}

func (a *Application) AvailableExchanges() []exchange.Meta {
	res := make([]exchange.Meta, len(a.exchanges))

	i := 0
	for _, e := range a.exchanges {
		res[i] = e.Meta()
		i++
	}

	sort.Slice(res, func(i, j int) bool {
		return res[i].Code < res[j].Code
	})

	return res
}
