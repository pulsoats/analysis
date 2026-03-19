package grpc

import analysispb "github.com/pulsoats/contracts/gen/go/analysis/v1"

type grpcWriter struct {
	stream analysispb.AnalysisService_GetRunResultServer
}

func (w *grpcWriter) Write(p []byte) (int, error) {
	buf := make([]byte, len(p))
	copy(buf, p)

	err := w.stream.Send(&analysispb.RunResultChunk{Data: buf})
	if err != nil {
		return 0, err
	}
	return len(p), nil
}
