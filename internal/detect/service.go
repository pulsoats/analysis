package detect

import (
	"context"

	"github.com/google/uuid"
	"github.com/pulsoats/analysis/internal/domain"
	"github.com/pulsoats/core/domain/detect"
	"github.com/pulsoats/core/domain/market"
)

type Service struct {
}

func NewDetectService() *Service {
	return &Service{}
}

func (svc *Service) Run(ctx context.Context, marketData []market.Candle, fees market.TakerMakerFees, detector detect.CandleDetector) ([]domain.AnalysisSignal, error) {
	res := make([]domain.AnalysisSignal, 0, 128)
	ws := detector.WindowSize()
	seen := make(map[uuid.UUID]struct{})
	for i := ws - 1; i < len(marketData); i++ {
		window := marketData[i-ws+1 : i+1]
		if err := ctx.Err(); err != nil {
			return nil, err
		}

		sig, ok, err := detector.Detect(ctx, window, fees)
		if err != nil {
			return nil, err
		}

		if !ok {
			continue
		}

		if _, exists := seen[sig.Fingerprint]; exists {
			continue
		}

		seen[sig.Fingerprint] = struct{}{}
		res = append(res, domain.AnalysisSignal{Signal: sig, Index: i})
	}

	return res, nil
}
