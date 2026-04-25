package analysis

import analysispb "github.com/pulsoats/contracts/gen/go/analysis/v1"

type grpcWriter struct {
	stream analysispb.Analysis_GetRunArchiveServer
}

func (w *grpcWriter) Write(p []byte) (int, error) {
	buf := make([]byte, len(p))
	copy(buf, p)

	if err := w.stream.Send(&analysispb.RunArchiveChunk{Data: buf}); err != nil {
		return 0, err
	}
	return len(p), nil
}
