package catalog

import (
	"fmt"
	"slices"

	"github.com/pulsoats/core/detect"
	"github.com/pulsoats/core/detect/detectors"
	"github.com/pulsoats/core/exchange"
)

type Application struct {
	detReg    *detectors.Registry
	exchanges map[string]exchange.PublicClient
}

func NewApplication(detectorsRegistry *detectors.Registry) (*Application, error) {
	if detectorsRegistry == nil {
		return nil, fmt.Errorf("detectors app: detectors registry is nil")
	}
	return &Application{detReg: detectorsRegistry}, nil
}

// ListAvailableDetectors возвращает слайс метаданных по каждому из встроенных детекторов.
// Метаданные берутся из реестра детекторов detectors.Registry.
func (a *Application) ListAvailableDetectors() []detect.DetectorMeta {
	return a.detReg.ListMetas()
}

func (a *Application) ListAvailableExchanges() []exchange.Meta {
	res := make([]exchange.Meta, 0, len(a.exchanges))
	for _, e := range a.exchanges {
		res = append(res, e.Meta())
	}

	slices.SortFunc(res, func(a, b exchange.Meta) int {
		if a.Code < b.Code {
			return -1
		}
		if a.Code > b.Code {
			return 1
		}
		return 0
	})

	return res
}
