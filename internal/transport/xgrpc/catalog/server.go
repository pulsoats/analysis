package catalog

import (
	"context"
	"fmt"

	"github.com/pulsoats/analysis/internal/application/catalog"
	catalogpb "github.com/pulsoats/contracts/gen/go/catalog/v1"
	"github.com/pulsoats/core/errorsx"
	"google.golang.org/protobuf/types/known/emptypb"
)

type Server struct {
	catalogpb.UnimplementedCatalogServer
	app *catalog.Application
}

func NewServer(catalogApp *catalog.Application) (*Server, error) {
	if catalogApp == nil {
		return nil, fmt.Errorf("grpc server: detector app: %w", errorsx.ErrInvalidArgument)
	}

	return &Server{app: catalogApp}, nil
}

func (s *Server) ListAvailableDetectors(_ context.Context, _ *emptypb.Empty) (*catalogpb.ListAvailableDetectorsResponse, error) {
	return mapToListAvailableDetectorsPb(s.app.AvailableDetectors()), nil
}

func (s *Server) ListAvailableFilters(_ context.Context, _ *emptypb.Empty) (*catalogpb.ListAvailableFiltersResponse, error) {
	return mapToListAvailableFiltersPb(s.app.AvailableFilters()), nil
}

func (s *Server) ListAvailableExchanges(_ context.Context, _ *emptypb.Empty) (*catalogpb.ListAvailableExchangesResponse, error) {
	return mapToListAvailableExchangesPb(s.app.AvailableExchanges()), nil
}
