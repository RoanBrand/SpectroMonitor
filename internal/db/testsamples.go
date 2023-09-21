package db

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/RoanBrand/SpectroMonitor/internal/model"
	"github.com/georgysavva/scany/v2/pgxscan"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type DBs struct {
	dbp *pgxpool.Pool
	ctx context.Context
}

func New(ctx context.Context, dbURL string) (*DBs, error) {
	dctx, cancel := context.WithTimeout(ctx, time.Minute*5)
	defer cancel()

	dbpool, err := pgxpool.New(dctx, dbURL)
	if err != nil {
		return nil, fmt.Errorf("unable to create db connection: %w", err)
	}

	return &DBs{dbpool, ctx}, nil
}

func (db *DBs) Close() error {
	db.dbp.Close()
	return nil
}

type dbTestSample struct {
	ID             int64
	TestTime       time.Time
	SpectroMachine int
	FurnaceName    string
	SampleName     string
}

func (db *DBs) ProcessResults(results []model.Result) error {
	ctx, cancel := context.WithTimeout(db.ctx, time.Second*5)
	defer cancel()

	err := pgx.BeginFunc(ctx, db.dbp, func(tx pgx.Tx) error {

		var lastResult dbTestSample
		err := pgxscan.Get(ctx, tx, &lastResult,
			`SELECT * FROM test_samples ORDER BY id DESC LIMIT 1;`)
		if err != nil && !errors.Is(err, pgx.ErrNoRows) {
			return fmt.Errorf("failed to scan for last inserted result: %w", err)
		}

		var tsId int64
		var rQry strings.Builder

		for i := len(results) - 1; i >= 0; i-- {
			r := &results[i]
			if r.TimeStamp.Before(lastResult.TestTime) {
				continue
			}

			if r.TimeStamp.Equal(lastResult.TestTime) && r.Spectro == lastResult.SpectroMachine {
				continue
			}

			err = tx.QueryRow(ctx,
				`INSERT INTO test_samples (test_time, spectro_machine, furnace_name, sample_name) VALUES ($1, $2, $3, $4) RETURNING id;`,
				r.TimeStamp, r.Spectro, r.Furnace, r.SampleName).Scan(&tsId)
			if err != nil {
				return fmt.Errorf("failed to add new test sample: %w", err)
			}

			args := make([]any, len(r.Results)+1)
			args[0] = tsId

			rQry.Reset()
			rQry.WriteString(`INSERT INTO sample_results (id`)
			for i, er := range r.Results {
				rQry.WriteString(`, "`)
				rQry.WriteString(er.Element)
				rQry.WriteString(`"`)

				args[i+1] = er.Value
			}

			rQry.WriteString(`) VALUES ($1`)
			for ern := range r.Results {
				rQry.WriteString(`, $`)
				rQry.WriteString(strconv.Itoa(ern + 2))
			}

			rQry.WriteString(`);`)

			_, err = tx.Exec(ctx, rQry.String(), args...)
			if err != nil {
				return fmt.Errorf("failed to add new test sample results: %w", err)
			}
		}
		return nil
	})

	return err
}
