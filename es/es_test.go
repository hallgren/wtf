package es_test

import (
	"context"
	"flag"
	"github.com/benbjohnson/wtf"
	"github.com/benbjohnson/wtf/sqlite"
	"io/ioutil"
	"path/filepath"
	"testing"
)

var dump = flag.Bool("dump", false, "save work data")

// MustOpenDB returns a new, open DB. Fatal on error.
func MustOpenDB(tb testing.TB) *sqlite.DB {
	tb.Helper()

	// Write to an in-memory database by default.
	// If the -dump flag is set, generate a temp file for the database.
	dsn := ":memory:"
	if *dump {
		dir, err := ioutil.TempDir("", "")
		if err != nil {
			tb.Fatal(err)
		}
		dsn = filepath.Join(dir, "db")
		println("DUMP=" + dsn)
	}

	db := sqlite.NewDB(dsn)
	if err := db.Open(); err != nil {
		tb.Fatal(err)
	}
	return db
}

// MustCloseDB closes the DB. Fatal on error.
func MustCloseDB(tb testing.TB, db *sqlite.DB) {
	tb.Helper()
	if err := db.Close(); err != nil {
		tb.Fatal(err)
	}
}

// MustCreateUser creates a user in the database. Fatal on error.
func MustCreateUser(tb testing.TB, ctx context.Context, db *sqlite.DB, user *wtf.User) (*wtf.User, context.Context) {
	tb.Helper()
	if err := sqlite.NewUserService(db).CreateUser(ctx, user); err != nil {
		tb.Fatal(err)
	}
	return user, wtf.NewContextWithUser(ctx, user)
}
