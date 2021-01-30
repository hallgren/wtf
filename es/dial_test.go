package es_test

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"

	"github.com/benbjohnson/wtf"
	"github.com/benbjohnson/wtf/es"
	"github.com/hallgren/eventsourcing"
	"github.com/hallgren/eventsourcing/eventstore/sql"
)

func TestDialService_CreateDial(t *testing.T) {
	// Ensure a dial can be created by a user & a membership for the user is automatically created.
	t.Run("OK", func(t *testing.T) {
		db := MustOpenDB(t)
		defer MustCloseDB(t, db)
		sqlDB := db.DB()

		ctx := context.Background()
		_, ctx0 := MustCreateUser(t, ctx, db, &wtf.User{Name: "jane", Email: "jane@gmail.com"})

		serializer := eventsourcing.NewSerializer(json.Marshal, json.Unmarshal)
		serializer.RegisterTypes(&wtf.Dial{},
			func() interface{} { return &wtf.Created{} },
			func() interface{} { return &wtf.SelfMembershipCreated{} },
		)
		e := sql.Open(sqlDB, *serializer)
		err := e.MigrateTest()
		if err != nil {
			t.Fatal(err)
		}
		repo := eventsourcing.NewRepository(e, nil)
		s := es.NewDialService(repo)
		dialCreate := &wtf.DialCreate{ID: 1, Name: "mydial"}

		// Create new dial. Ensure the current user is the owner & an invite code is generated.
		dial, err := s.CreateDial(ctx0, dialCreate)
		fmt.Println(dial)
		if err != nil {
			t.Fatal(err)
		} else if got, want := dial.ID, 1; got != want {
			t.Fatalf("ID=%v, want %v", got, want)
		} else if got, want := dial.UserID, 1; got != want {
			t.Fatalf("UserID=%v, want %v", got, want)
		} else if dial.InviteCode == "" {
			t.Fatal("expected invite code generation")
		} else if dial.CreatedAt.IsZero() {
			t.Fatal("expected created at")
		} else if dial.UpdatedAt.IsZero() {
			t.Fatal("expected updated at")
		}

		// Fetch dial from database & compare.
		other, err := s.FindDialByID(ctx0, dial.ID)
		if err != nil {
			t.Fatal(err)
		}
		fmt.Println(other)
		/*

			// Ensure membership for owner automatically created.
			if _, n, err := sqlite.NewDialMembershipService(db).FindDialMemberships(ctx0, wtf.DialMembershipFilter{DialID: &dial.ID}); err != nil {
				t.Fatal(err)
			} else if n != 1 {
				t.Fatal("expected owner membership auto-creation")
			}
		*/
	})
}
