package catalog

import (
	"context"
	"fmt"

	catalogpb "github.com/pulsoats/contracts/gen/go/catalog/v1"
	"github.com/pulsoats/core/detect"
	"github.com/pulsoats/core/errorsx"
	"github.com/pulsoats/core/exchange"
	"google.golang.org/protobuf/types/known/emptypb"
)

type app interface {
	ListAvailableDetectors() []detect.DetectorMeta
	ListAvailableExchanges() []exchange.Meta
}
type Server struct {
	catalogpb.UnimplementedCatalogServer
	app app
}

func NewServer(catalogApp app) (*Server, error) {
	if catalogApp == nil {
		return nil, fmt.Errorf("grpc server: detector app: %w", errorsx.ErrInvalidArgument)
	}

	return &Server{app: catalogApp}, nil
}

func (s *Server) ListAvailableDetectors(_ context.Context, _ *emptypb.Empty) (*catalogpb.ListAvailableDetectorsResponse, error) {
	return mapToListAvailableDetectorsPb(s.app.ListAvailableDetectors()), nil
}

func (s *Server) ListAvailableExchanges(_ context.Context, _ *emptypb.Empty) (*catalogpb.ListAvailableExchangesResponse, error) {
	return mapToListAvailableExchangesPb(s.app.ListAvailableExchanges()), nil
}
