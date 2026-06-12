package runs

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/pulsoats/analysis/internal/domain/run"
	"github.com/pulsoats/analysis/internal/infrastructure/repository/postgres"
	"github.com/pulsoats/core/errorsx"
	"github.com/pulsoats/core/market"
)

type repo struct {
	qp postgres.QuerierProvider
}

func NewRepository(qp postgres.QuerierProvider) run.Repository {
	return &repo{qp: qp}
}

func (r *repo) CreateRun(ctx context.Context, run *run.Run) error {
	const query = `
		INSERT INTO analysis.runs (
			id, exchange, category, symbol, interval,
			detector_code, detector_version, detector_label, detector_opts,
			first_candle_time, last_candle_time, status_code, status_message, created_by
		)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14)
		RETURNING created_at;
	`

	q := r.qp.Get(ctx)

	err := q.QueryRow(ctx, query,
		run.ID,
		run.Market.Exchange,
		run.Market.Category,
		run.Market.Symbol,
		run.Interval.String(),
		run.Detector.Code,
		run.Detector.Version,
		run.Detector.OptsLabel,
		run.Detector.Opts,
		run.FirstCandleTime,
		run.LastCandleTime,
		run.Status.Code,
		run.Status.Message,
		run.CreatedBy,
	).Scan(&run.CreatedAt)
	if err != nil {
		return fmt.Errorf("create run: %w", errors.Join(errorsx.ErrInternal, err))
	}
	return nil
}

func (r *repo) UpdateRun(ctx context.Context, res run.Run) error {
	const query = `
		UPDATE analysis.runs
		SET status_code    = $2,
		    status_message = $3,
		    first_candle_time      = $4,
		    last_candle_time        = $5,
		    signals_count  = $6,
		    avg_profit_ppm = $7,
		    is_shared      = $8,
		    shared_at      = $9
		WHERE id = $1;
	`

	q := r.qp.Get(ctx)

	tag, err := q.Exec(ctx, query,
		res.ID,
		res.Status.Code,
		res.Status.Message,
		res.FirstCandleTime,
		res.LastCandleTime,
		res.SignalsCount,
		res.AvgProfitPPM,
		res.IsShared,
		res.SharedAt,
	)
	if err != nil {
		return fmt.Errorf("update run: %w", errors.Join(errorsx.ErrInternal, err))
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("update run: %w", errorsx.ErrNotFound)
	}
	return nil
}

func (r *repo) RunByID(ctx context.Context, runID uuid.UUID) (run.Run, error) {
	const query = `
		SELECT
			id, exchange, category, symbol, interval,
			detector_code, detector_version, detector_label, detector_opts,
			first_candle_time, last_candle_time, signals_count, avg_profit_ppm,
			created_by, created_at, status_code, status_message, is_shared, shared_at
		FROM analysis.runs
		WHERE id = $1;
	`

	q := r.qp.Get(ctx)

	var res run.Run
	var interval string
	err := q.QueryRow(ctx, query, runID).Scan(
		&res.ID,
		&res.Market.Exchange,
		&res.Market.Category,
		&res.Market.Symbol,
		&interval,
		&res.Detector.Code,
		&res.Detector.Version,
		&res.Detector.OptsLabel,
		&res.Detector.Opts,
		&res.FirstCandleTime,
		&res.LastCandleTime,
		&res.SignalsCount,
		&res.AvgProfitPPM,
		&res.CreatedBy,
		&res.CreatedAt,
		&res.Status.Code,
		&res.Status.Message,
		&res.IsShared,
		&res.SharedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return run.Run{}, fmt.Errorf("get run: %w", errorsx.ErrNotFound)
		}
		return run.Run{}, fmt.Errorf("get run: %w", errors.Join(errorsx.ErrInternal, err))
	}
	if iv, ok := market.ParseInterval(interval); ok {
		res.Interval = iv
	}
	return res, nil
}

func (r *repo) RunsPaged(ctx context.Context, req run.RunsPagedRequest) (run.RunsPagedResponse, error) {
	const baseQuery = `
	SELECT
    	id, exchange, category, symbol, interval,
		detector_code, detector_version, detector_label, detector_opts,
		first_candle_time, last_candle_time, signals_count, avg_profit_ppm,
		created_by, created_at, status_code, status_message, is_shared, shared_at
	FROM analysis.runs
	WHERE 1 = 1`

	if req.Limit <= 0 {
		return run.RunsPagedResponse{}, fmt.Errorf("runs paged: limit %d: %w", req.Limit, errorsx.ErrInvalidArgument)
	}

	query := baseQuery
	var args []any
	argN := 1

	var scopeQuery string
	switch req.Scope {
	case run.ScopeShared:
		scopeQuery = fmt.Sprintf(" AND is_shared = true AND created_by != $%d", argN)
	case run.ScopeAll:
		scopeQuery = fmt.Sprintf(" AND (created_by = $%d OR is_shared = true)", argN)
	default:
		scopeQuery = fmt.Sprintf(" AND created_by = $%d", argN)
	}

	query += scopeQuery
	args = append(args, req.UserID)
	argN++

	if req.BeforeID != nil {
		if req.OrderDirAsc {
			query += fmt.Sprintf(" AND id > $%d", argN)
		} else {
			query += fmt.Sprintf(" AND id < $%d", argN)
		}
		args = append(args, req.BeforeID)
		argN++
	}

	if req.Filter != nil {
		if len(req.Filter.Exchanges) > 0 {
			query += fmt.Sprintf(" AND exchange = ANY($%d)", argN)
			args = append(args, req.Filter.Exchanges)
			argN++
		}

		if len(req.Filter.Categories) > 0 {
			query += fmt.Sprintf(" AND category = ANY($%d)", argN)
			args = append(args, req.Filter.Categories)
			argN++
		}

		if len(req.Filter.Symbols) > 0 {
			query += fmt.Sprintf(" AND symbol = ANY($%d)", argN)
			args = append(args, req.Filter.Symbols)
			argN++
		}

		if len(req.Filter.Intervals) > 0 {
			query += fmt.Sprintf(" AND interval = ANY($%d)", argN)
			args = append(args, req.Filter.Intervals)
			argN++
		}

		if len(req.Filter.DetectorCodes) > 0 {
			query += fmt.Sprintf(" AND detector_code = ANY($%d)", argN)
			args = append(args, req.Filter.DetectorCodes)
			argN++
		}

		if len(req.Filter.Statuses) > 0 {
			query += fmt.Sprintf(" AND status_code = ANY($%d)", argN)
			args = append(args, req.Filter.Statuses)
			argN++
		}

		if req.Filter.MinSignals != nil {
			query += fmt.Sprintf(" AND signals_count >= $%d", argN)
			args = append(args, req.Filter.MinSignals)
			argN++
		}

		if req.Filter.MaxSignals != nil {
			query += fmt.Sprintf(" AND signals_count <= $%d", argN)
			args = append(args, req.Filter.MaxSignals)
			argN++
		}

		if req.Filter.MinAvgProfitPPM != nil {
			query += fmt.Sprintf(" AND avg_profit_ppm >= $%d", argN)
			args = append(args, req.Filter.MinAvgProfitPPM)
			argN++
		}

		if req.Filter.MaxAvgProfitPPM != nil {
			query += fmt.Sprintf(" AND avg_profit_ppm <= $%d", argN)
			args = append(args, req.Filter.MaxAvgProfitPPM)
			argN++
		}

		if req.Filter.FirstCandleFrom != nil {
			query += fmt.Sprintf(" AND first_candle_time >= $%d", argN)
			args = append(args, req.Filter.FirstCandleFrom)
			argN++
		}

		if req.Filter.LastCandleTo != nil {
			query += fmt.Sprintf(" AND last_candle_time <= $%d", argN)
			args = append(args, req.Filter.LastCandleTo)
			argN++
		}

		if req.Filter.CreatedFrom != nil {
			query += fmt.Sprintf(" AND created_at >= $%d", argN)
			args = append(args, req.Filter.CreatedFrom)
			argN++
		}

		if req.Filter.CreatedTo != nil {
			query += fmt.Sprintf(" AND created_at <= $%d", argN)
			args = append(args, req.Filter.CreatedTo)
			argN++
		}
	}

	orderQuery := " ORDER BY id DESC"
	if req.OrderDirAsc {
		orderQuery = " ORDER BY id ASC"
	}
	query += orderQuery

	query += fmt.Sprintf(" LIMIT $%d", argN)
	args = append(args, req.Limit+1)

	q := r.qp.Get(ctx)

	rows, err := q.Query(ctx, query, args...)
	if err != nil {
		return run.RunsPagedResponse{}, fmt.Errorf("runs paged: %w", errors.Join(errorsx.ErrInternal, err))
	}
	defer rows.Close()

	runs, err := pgx.CollectRows(rows, func(row pgx.CollectableRow) (run.Run, error) {
		var res run.Run
		var rawInterval string

		if err := row.Scan(
			&res.ID,
			&res.Market.Exchange,
			&res.Market.Category,
			&res.Market.Symbol,
			&rawInterval,
			&res.Detector.Code,
			&res.Detector.Version,
			&res.Detector.OptsLabel,
			&res.Detector.Opts,
			&res.FirstCandleTime,
			&res.LastCandleTime,
			&res.SignalsCount,
			&res.AvgProfitPPM,
			&res.CreatedBy,
			&res.CreatedAt,
			&res.Status.Code,
			&res.Status.Message,
			&res.IsShared,
			&res.SharedAt,
		); err != nil {
			return run.Run{}, err
		}

		if iv, ok := market.ParseInterval(rawInterval); ok {
			res.Interval = iv
		}

		return res, nil
	})
	if err != nil {
		return run.RunsPagedResponse{}, fmt.Errorf("collect runs paged: %w", errors.Join(errorsx.ErrInternal, err))
	}

	hasMore := len(runs) > req.Limit
	if hasMore {
		runs = runs[:req.Limit]
	}

	var nextBeforeID *uuid.UUID
	if hasMore && len(runs) > 0 {
		lastID := runs[len(runs)-1].ID
		nextBeforeID = &lastID
	}

	return run.RunsPagedResponse{
		Runs:         runs,
		HasMore:      hasMore,
		NextBeforeID: nextBeforeID,
	}, nil
}

func (r *repo) ShareRun(ctx context.Context, runID uuid.UUID, callerID string) error {
	const query = `
		UPDATE analysis.runs
		SET is_shared = true, shared_at = NOW()
		WHERE id = $1 AND created_by = $2;
	`

	q := r.qp.Get(ctx)

	tag, err := q.Exec(ctx, query, runID, callerID)
	if err != nil {
		return fmt.Errorf("share run: %w", errors.Join(errorsx.ErrInternal, err))
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("share run: %w", errorsx.ErrNotFound)
	}
	return nil
}

func (r *repo) DeleteRun(ctx context.Context, runID uuid.UUID) error {
	const query = `
		DELETE FROM analysis.runs
		WHERE id = $1;
	`

	q := r.qp.Get(ctx)

	tag, err := q.Exec(ctx, query, runID)
	if err != nil {
		return fmt.Errorf("delete run: %w", errors.Join(errorsx.ErrInternal, err))
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("delete run: %w", errorsx.ErrNotFound)
	}
	return nil
}
