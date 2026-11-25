package migrations

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"errors"
	"fmt"
	"regexp"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
)

func TestApply(t *testing.T) {
	for _, tc := range []struct {
		name                   string
		seed, existing, seeded bool
		failure                string
	}{
		{name: "fresh"},
		{name: "seed", seed: true},
		{name: "idempotent", seed: true, existing: true, seeded: true},
		{name: "begin failure", failure: "begin"},
		{name: "lock failure", failure: "lock"},
		{name: "metadata failure", failure: "metadata"},
		{name: "lookup failure", failure: "lookup"},
		{name: "checksum failure", existing: true, failure: "checksum"},
		{name: "migration failure", failure: "migration"},
		{name: "record failure", failure: "record"},
		{name: "seed check failure", seed: true, failure: "seedcheck"},
		{name: "seed failure", seed: true, failure: "seed"},
		{name: "commit failure", failure: "commit"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			db, mock, err := sqlmock.New()
			if err != nil {
				t.Fatal(err)
			}
			defer db.Close()
			errBoom := errors.New("database error")
			prepare := func() {
				begin := mock.ExpectBegin()
				if tc.failure == "begin" {
					begin.WillReturnError(errBoom)
					return
				}
				lock := mock.ExpectExec("SELECT pg_advisory_xact_lock")
				if tc.failure == "lock" {
					lock.WillReturnError(errBoom)
					return
				}
				lock.WillReturnResult(sqlmock.NewResult(0, 0))
				metadata := mock.ExpectExec("CREATE TABLE IF NOT EXISTS schema_migrations")
				if tc.failure == "metadata" {
					metadata.WillReturnError(errBoom)
					return
				}
				metadata.WillReturnResult(sqlmock.NewResult(0, 0))
				for _, name := range []string{"001_init.sql", "002_site_content.sql"} {
					body, err := files.ReadFile(name)
					if err != nil {
						t.Fatal(err)
					}
					checksum := fmt.Sprintf("%x", sha256.Sum256(body))
					query := mock.ExpectQuery(regexp.QuoteMeta("SELECT checksum FROM schema_migrations WHERE name = $1")).WithArgs(name)
					if tc.failure == "lookup" {
						query.WillReturnError(errBoom)
						return
					}
					if tc.existing {
						if tc.failure == "checksum" {
							query.WillReturnRows(sqlmock.NewRows([]string{"checksum"}).AddRow("changed"))
							return
						}
						query.WillReturnRows(sqlmock.NewRows([]string{"checksum"}).AddRow(checksum))
						continue
					}
					query.WillReturnError(sql.ErrNoRows)
					migration := mock.ExpectExec(regexp.QuoteMeta(string(body)))
					if tc.failure == "migration" {
						migration.WillReturnError(errBoom)
						return
					}
					migration.WillReturnResult(sqlmock.NewResult(0, 0))
					record := mock.ExpectExec("INSERT INTO schema_migrations").WithArgs(name, checksum)
					if tc.failure == "record" {
						record.WillReturnError(errBoom)
						return
					}
					record.WillReturnResult(sqlmock.NewResult(0, 1))
				}
				if tc.seed {
					check := mock.ExpectQuery("SELECT EXISTS")
					if tc.failure == "seedcheck" {
						check.WillReturnError(errBoom)
						return
					}
					check.WillReturnRows(sqlmock.NewRows([]string{"exists"}).AddRow(tc.seeded))
					if !tc.seeded {
						body, err := files.ReadFile("seed.sql")
						if err != nil {
							t.Fatal(err)
						}
						seed := mock.ExpectExec(regexp.QuoteMeta(string(body)))
						if tc.failure == "seed" {
							seed.WillReturnError(errBoom)
							return
						}
						seed.WillReturnResult(sqlmock.NewResult(0, 1))
					}
				}
				commit := mock.ExpectCommit()
				if tc.failure == "commit" {
					commit.WillReturnError(errBoom)
				}
			}
			prepare()
			if tc.failure != "" && tc.failure != "begin" && tc.failure != "commit" {
				mock.ExpectRollback()
			}
			err = Apply(context.Background(), db, tc.seed)
			if (err != nil) != (tc.failure != "") {
				t.Fatalf("err=%v", err)
			}
			if err := mock.ExpectationsWereMet(); err != nil {
				t.Fatal(err)
			}
		})
	}
}
