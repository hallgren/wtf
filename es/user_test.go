package es_test

import (
	"context"
	"flag"
	"io/ioutil"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/benbjohnson/wtf"
	"github.com/benbjohnson/wtf/es"
	"github.com/benbjohnson/wtf/sqlite"
)

func TestUserService_CreateUser(t *testing.T) {
	// Ensure user can be created.
	t.Run("OK", func(t *testing.T) {
		db := MustOpenDB(t)
		defer MustCloseDB(t, db)
		us := sqlite.NewUserService(db)
		s := es.NewUserService(us)

		u := &wtf.User{
			Name:  "susy",
			Email: "susy@gmail.com",
		}

		// Create new user & verify ID and timestamps are set.
		if err := s.CreateUser(context.Background(), u); err != nil {
			t.Fatal(err)
		} else if got, want := u.ID, 1; got != want {
			t.Fatalf("ID=%v, want %v", got, want)
		} else if u.CreatedAt.IsZero() {
			t.Fatal("expected created at")
		} else if u.UpdatedAt.IsZero() {
			t.Fatal("expected updated at")
		}

		// Create second user with email.
		u2 := &wtf.User{Name: "jane"}
		if err := s.CreateUser(context.Background(), u2); err != nil {
			t.Fatal(err)
		} else if got, want := u2.ID, 2; got != want {
			t.Fatalf("ID=%v, want %v", got, want)
		}

		// Fetch user from database & compare.
		if other, err := s.FindUserByID(context.Background(), 1); err != nil {
			t.Fatal(err)
		} else if !reflect.DeepEqual(u, other) {
			t.Fatalf("mismatch: %#v != %#v", u, other)
		}
	})

	// Ensure an error is returned if user name is not set.
	t.Run("ErrNameRequired", func(t *testing.T) {
		db := MustOpenDB(t)
		defer MustCloseDB(t, db)
		us := sqlite.NewUserService(db)
		s := es.NewUserService(us)
		if err := s.CreateUser(context.Background(), &wtf.User{}); err == nil {
			t.Fatal("expected error")
		} else if wtf.ErrorCode(err) != wtf.EINVALID || wtf.ErrorMessage(err) != `User name required.` {
			t.Fatalf("unexpected error: %#v", err)
		}
	})
}

func TestUserService_UpdateUser(t *testing.T) {
	// Ensure user name & email can be updated by current user.
	t.Run("OK", func(t *testing.T) {
		db := MustOpenDB(t)
		defer MustCloseDB(t, db)
		us := sqlite.NewUserService(db)
		s := es.NewUserService(us)
		user0, ctx0 := MustCreateUser(t, context.Background(), db, &wtf.User{
			Name:  "susy",
			Email: "susy@gmail.com",
		})

		// Update user.
		newName, newEmail := "jill", "jill@gmail.com"
		uu, err := s.UpdateUser(ctx0, user0.ID, wtf.UserUpdate{
			Name:  &newName,
			Email: &newEmail,
		})
		if err != nil {
			t.Fatal(err)
		} else if got, want := uu.Name, "jill"; got != want {
			t.Fatalf("Name=%v, want %v", got, want)
		} else if got, want := uu.Email, "jill@gmail.com"; got != want {
			t.Fatalf("Email=%v, want %v", got, want)
		}

		// Fetch user from database & compare.
		if other, err := s.FindUserByID(context.Background(), 1); err != nil {
			t.Fatal(err)
		} else if !reflect.DeepEqual(uu, other) {
			t.Fatalf("mismatch: %#v != %#v", uu, other)
		}
	})

	// Ensure updating a user is restricted only to the current user.
	t.Run("ErrUnauthorized", func(t *testing.T) {
		db := MustOpenDB(t)
		defer MustCloseDB(t, db)
		us := sqlite.NewUserService(db)
		s := es.NewUserService(us)
		user0, _ := MustCreateUser(t, context.Background(), db, &wtf.User{Name: "NAME0"})
		_, ctx1 := MustCreateUser(t, context.Background(), db, &wtf.User{Name: "NAME1"})

		// Update user as another user.
		newName := "NEWNAME"
		if _, err := s.UpdateUser(ctx1, user0.ID, wtf.UserUpdate{Name: &newName}); err == nil {
			t.Fatal("expected error")
		} else if wtf.ErrorCode(err) != wtf.EUNAUTHORIZED || wtf.ErrorMessage(err) != `You are not allowed to update this user.` {
			t.Fatalf("unexpected error: %#v", err)
		}
	})
}

var dump = flag.Bool("dump", false, "save work data")

// Ensure the test database can open & close.
func TestDB(t *testing.T) {
	db := MustOpenDB(t)
	MustCloseDB(t, db)
}

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
