package runs

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/pulsoats/analysis/internal/domain/run"
	"github.com/pulsoats/analysis/internal/infrastructure/repository/postgres"
	"github.com/pulsoats/core/domain/market"
	"github.com/pulsoats/core/errorsx"
)

type repo struct {
	qp postgres.QuerierProvider
}

func NewRepository(qp postgres.QuerierProvider) run.Repository {
	return &repo{qp: qp}
}

func (r *repo) CreateRun(ctx context.Context, runModel *run.Run, status run.Status) error {
	const query = `
			INSERT INTO analysis.runs (
				exchange, category, symbol, interval,
			detector_code, detector_label, detector_opts,
			from_time, to_time, status_code, status_message,
			created_by, price_type
			)
			VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13)
			RETURNING id, created_at;
	`

	q := r.qp.Get(ctx)

	err := q.QueryRow(ctx, query,
		runModel.Market.Exchange,
		runModel.Market.Category,
		runModel.Market.Symbol,
		runModel.Interval.String(),
		runModel.Detector.Code,
		runModel.Detector.Label,
		runModel.Detector.Opts,
		runModel.From,
		runModel.To,
		status.Code,
		status.Message,
		runModel.CreatedBy,
		runModel.PriceType,
	).Scan(&runModel.ID, &runModel.CreatedAt)
	if err != nil {
		return fmt.Errorf("create run: %w", errors.Join(errorsx.ErrInternal, err))
	}
	runModel.Status = status
	return nil
}

func (r *repo) UpdateStatus(ctx context.Context, runID int64, status run.Status) error {
	const query = `
		UPDATE analysis.runs
		SET status_code = $2, status_message = $3
		WHERE id = $1;
	`

	q := r.qp.Get(ctx)

	tag, err := q.Exec(ctx, query, runID, status.Code, status.Message)
	if err != nil {
		return fmt.Errorf("update run status: %w", errors.Join(errorsx.ErrInternal, err))
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("update run status: %w", errorsx.ErrNotFound)
	}
	return nil
}

func (r *repo) UpdateResult(ctx context.Context, res run.Run) error {
	const query = `
		UPDATE analysis.runs
		SET from_time = $2,
		    to_time = $3,
		    signals_count = $4,
		    avg_profit_ppm = $5
		WHERE id = $1;
	`

	q := r.qp.Get(ctx)

	tag, err := q.Exec(ctx, query,
		res.ID,
		res.From,
		res.To,
		res.SignalsCount,
		res.AvgProfitPPM,
	)
	if err != nil {
		return fmt.Errorf("update run result: %w", errors.Join(errorsx.ErrInternal, err))
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("update run result: %w", errorsx.ErrNotFound)
	}
	return nil
}

func (r *repo) StatusByRunID(ctx context.Context, runID int64) (run.Status, error) {
	const query = `
		SELECT status_code, status_message
		FROM analysis.runs
		WHERE id = $1;
	`

	q := r.qp.Get(ctx)

	var st run.Status
	err := q.QueryRow(ctx, query, runID).Scan(&st.Code, &st.Message)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return run.Status{}, fmt.Errorf("get run status: %w", errorsx.ErrNotFound)
		}
		return run.Status{}, fmt.Errorf("get run status: %w", errors.Join(errorsx.ErrInternal, err))
	}
	return st, nil
}

func (r *repo) RunByID(ctx context.Context, runID int64) (run.Run, error) {
	const query = `
		SELECT
			id,
			exchange,
			category,
			symbol,
			interval,
			price_type,
			detector_code,
			detector_label,
			detector_opts,
			from_time,
			to_time,
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
		&res.PriceType,
		&res.Detector.Code,
		&res.Detector.Label,
		&res.Detector.Opts,
		&res.From,
		&res.To,
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

func (r *repo) ListRunsPaged(ctx context.Context, limit int, beforeID *int64, callerID string, filter run.Filter) ([]run.Run, bool, *int64, error) {
	// $1 = beforeID, $2 = limit+1, $3 = callerID
	const (
		queryMine = `
			SELECT
				id, exchange, category, symbol, interval, price_type,
				detector_code, detector_label, detector_opts,
				from_time, to_time, signals_count, avg_profit_ppm,
				created_by, created_at, status_code, status_message,
				is_shared, shared_at
			FROM analysis.runs
			WHERE created_by = $3
			  AND status_code IN (3, 4)
			  AND ($1::bigint IS NULL OR id < $1)
			ORDER BY id DESC
			LIMIT $2;`
		queryShared = `
			SELECT
				id, exchange, category, symbol, interval, price_type,
				detector_code, detector_label, detector_opts,
				from_time, to_time, signals_count, avg_profit_ppm,
				created_by, created_at, status_code, status_message,
				is_shared, shared_at
			FROM analysis.runs
			WHERE is_shared = true AND created_by != $3
			  AND status_code IN (3, 4)
			  AND ($1::bigint IS NULL OR id < $1)
			ORDER BY id DESC
			LIMIT $2;`
		queryAll = `
			SELECT
				id, exchange, category, symbol, interval, price_type,
				detector_code, detector_label, detector_opts,
				from_time, to_time, signals_count, avg_profit_ppm,
				created_by, created_at, status_code, status_message,
				is_shared, shared_at
			FROM analysis.runs
			WHERE (created_by = $3 OR is_shared = true)
			  AND status_code IN (3, 4)
			  AND ($1::bigint IS NULL OR id < $1)
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
			&newRun.PriceType,
			&newRun.Detector.Code,
			&newRun.Detector.Label,
			&newRun.Detector.Opts,
			&newRun.From,
			&newRun.To,
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

	var nextBeforeID *int64
	if hasMore && len(runs) > 0 {
		lastID := runs[len(runs)-1].ID
		nextBeforeID = &lastID
	}

	return runs, hasMore, nextBeforeID, nil
}

func (r *repo) ShareRun(ctx context.Context, runID int64, callerID string) error {
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

func (r *repo) DeleteRun(ctx context.Context, runID int64) error {
	const query = `
	DELETE
	FROM analysis.runs
	WHERE id = $1
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
