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
			detector_code, detector_label, detector_opts,
			first_candle_time, last_candle_time, status_code, status_message, created_by
		)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13)
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
			id,
			exchange,
			category,
			symbol,
			interval,
			detector_code,
			detector_label,
			detector_opts,
			first_candle_time,
			last_candle_time,
			signals_count,
			avg_profit_ppm,
			created_by,
			created_at,
			status_code,
			status_message,
			is_shared,
			shared_at
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

func (r *repo) ListRunsPaged(ctx context.Context, limit int, beforeID *uuid.UUID, callerID string, filter run.Filter) ([]run.Run, bool, *uuid.UUID, error) {
	if limit <= 0 {
		return nil, false, nil, fmt.Errorf("list runs paged: limit %d: %w", limit, errorsx.ErrInvalidArgument)
	}

	// $1 = beforeID, $2 = limit+1, $3 = callerID
	const (
		queryMine = `
			SELECT
				id, exchange, category, symbol, interval,
				detector_code, detector_label, detector_opts,
				first_candle_time, last_candle_time, signals_count, avg_profit_ppm,
				created_by, created_at, status_code, status_message,
				is_shared, shared_at
			FROM analysis.runs
			WHERE created_by = $3
			  AND status_code IN (3, 4)
			  AND ($1::uuid IS NULL OR id < $1)
			ORDER BY id DESC
			LIMIT $2;`
		queryShared = `
			SELECT
				id, exchange, category, symbol, interval,
				detector_code, detector_label, detector_opts,
				first_candle_time, last_candle_time, signals_count, avg_profit_ppm,
				created_by, created_at, status_code, status_message,
				is_shared, shared_at
			FROM analysis.runs
			WHERE is_shared = true AND created_by != $3
			  AND status_code IN (3, 4)
			  AND ($1::uuid IS NULL OR id < $1)
			ORDER BY id DESC
			LIMIT $2;`
		queryAll = `
			SELECT
				id, exchange, category, symbol, interval,
				detector_code, detector_label, detector_opts,
				first_candle_time, last_candle_time, signals_count, avg_profit_ppm,
				created_by, created_at, status_code, status_message,
				is_shared, shared_at
			FROM analysis.runs
			WHERE (created_by = $3 OR is_shared = true)
			  AND status_code IN (3, 4)
			  AND ($1::uuid IS NULL OR id < $1)
			ORDER BY id DESC
			LIMIT $2;`
	)

	q := r.qp.Get(ctx)

	query := queryMine
	switch filter {
	case run.FilterShared:
		query = queryShared
	case run.FilterAll:
		query = queryAll
	}

	rows, err := q.Query(ctx, query, beforeID, limit+1, callerID)
	if err != nil {
		return nil, false, nil, fmt.Errorf("list runs paged: %w", errors.Join(errorsx.ErrInternal, err))
	}
	defer rows.Close()

	runs, err := pgx.CollectRows(rows, func(row pgx.CollectableRow) (run.Run, error) {
		var newRun run.Run
		var rawInterval string

		if err := row.Scan(
			&newRun.ID,
			&newRun.Market.Exchange,
			&newRun.Market.Category,
			&newRun.Market.Symbol,
			&rawInterval,
			&newRun.Detector.Code,
			&newRun.Detector.OptsLabel,
			&newRun.Detector.Opts,
			&newRun.FirstCandleTime,
			&newRun.LastCandleTime,
			&newRun.SignalsCount,
			&newRun.AvgProfitPPM,
			&newRun.CreatedBy,
			&newRun.CreatedAt,
			&newRun.Status.Code,
			&newRun.Status.Message,
			&newRun.IsShared,
			&newRun.SharedAt,
		); err != nil {
			return run.Run{}, err
		}

		if iv, ok := market.ParseInterval(rawInterval); ok {
			newRun.Interval = iv
		}

		return newRun, nil
	})
	if err != nil {
		return nil, false, nil, fmt.Errorf("collect runs paged: %w", errors.Join(errorsx.ErrInternal, err))
	}

	hasMore := len(runs) > limit
	if hasMore {
		runs = runs[:limit]
	}

	var nextBeforeID *uuid.UUID
	if hasMore && len(runs) > 0 {
		lastID := runs[len(runs)-1].ID
		nextBeforeID = &lastID
	}

	return runs, hasMore, nextBeforeID, nil
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
