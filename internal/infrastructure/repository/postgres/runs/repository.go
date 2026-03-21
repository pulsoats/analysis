package runs

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/pulsoats/analysis/internal/model/run"
	"github.com/pulsoats/core/domain/derrors"
	"github.com/pulsoats/core/domain/market"
	"github.com/pulsoats/core/lib/errorsx"
)

type repo struct {
	pool *pgxpool.Pool
}

func NewRepository(pool *pgxpool.Pool) run.Repository {
	return &repo{pool: pool}
}

func (r *repo) CreateRun(ctx context.Context, runModel *run.Run, status run.Status) error {
	const query = `
			INSERT INTO analysis.runs (
				exchange, category, symbol, interval,
			detector_code, detector_label, detector_opts,
			from_time, to_time, status_code, status_message,
			created_by
			)
			VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12)
			RETURNING id, created_at;
	`
	err := r.pool.QueryRow(ctx, query,
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
	).Scan(&runModel.ID, &runModel.CreatedAt)
	if err != nil {
		return fmt.Errorf("create run: %w: %v", errorsx.ErrInternal, err)
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
	tag, err := r.pool.Exec(ctx, query, runID, status.Code, status.Message)
	if err != nil {
		return fmt.Errorf("update run status: %w: %v", errorsx.ErrInternal, err)
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("update run status: %w", derrors.ErrNotFound)
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
	tag, err := r.pool.Exec(ctx, query,
		res.ID,
		nullTime(res.From),
		nullTime(res.To),
		res.SignalsCount,
		res.AvgProfitPPM,
	)
	if err != nil {
		return fmt.Errorf("update run result: %w: %v", errorsx.ErrInternal, err)
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("update run result: %w", derrors.ErrNotFound)
	}
	return nil
}

func (r *repo) StatusByRunID(ctx context.Context, runID int64) (run.Status, error) {
	const query = `
		SELECT status_code, status_message
		FROM analysis.runs
		WHERE id = $1;
	`
	var st run.Status
	err := r.pool.QueryRow(ctx, query, runID).Scan(&st.Code, &st.Message)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return run.Status{}, fmt.Errorf("get run status: %w", derrors.ErrNotFound)
		}
		return run.Status{}, fmt.Errorf("get run status: %w: %v", errorsx.ErrInternal, err)
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
			status_message
		FROM analysis.runs
		WHERE id = $1;
	`

	var res run.Run
	var interval string
	err := r.pool.QueryRow(ctx, query, runID).Scan(
		&res.ID,
		&res.Market.Exchange,
		&res.Market.Category,
		&res.Market.Symbol,
		&interval,
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
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return run.Run{}, fmt.Errorf("get run: %w", derrors.ErrNotFound)
		}
		return run.Run{}, fmt.Errorf("get run: %w: %v", errorsx.ErrInternal, err)
	}
	if iv, ok := market.ParseInterval(interval); ok {
		res.Interval = iv
	}
	return res, nil
}

func (r *repo) ListRunsPaged(ctx context.Context, limit int, beforeID *int64) ([]run.Run, bool, *int64, error) {
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
			from_time,
			to_time,
			signals_count,
			avg_profit_ppm,
			created_by,
			created_at,
			status_code,
			status_message
		FROM analysis.runs
		WHERE ($1::bigint IS NULL OR id < $1)
		ORDER BY id DESC
		LIMIT $2;
	`

	rows, err := r.pool.Query(ctx, query, beforeID, limit+1)
	if err != nil {
		return nil, false, nil, fmt.Errorf("list runs paged: %w: %v", errorsx.ErrInternal, err)
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
		); err != nil {
			return run.Run{}, err
		}

		if iv, ok := market.ParseInterval(rawInterval); ok {
			newRun.Interval = iv
		}

		return newRun, nil
	})
	if err != nil {
		return nil, false, nil, fmt.Errorf("collect runs paged: %w: %v", errorsx.ErrInternal, err)
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

func nullTime(t time.Time) *time.Time {
	if t.IsZero() {
		return nil
	}
	return &t
}
