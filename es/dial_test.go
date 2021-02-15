package es_test

import (
	"context"
	"testing"

	"github.com/benbjohnson/wtf"
	"github.com/benbjohnson/wtf/es"
	"github.com/benbjohnson/wtf/sqlite"
	"github.com/hallgren/eventsourcing"
	"github.com/hallgren/eventsourcing/eventstore/memory"
)

func TestCreateDial(t *testing.T) {
	m := memory.Create()
	defer m.Close()
	db := MustOpenDB(t)

	repo := eventsourcing.NewRepository(m, nil)
	sqlDialSerive := sqlite.NewDialService(db)
	dialService := es.NewDialService(repo, sqlDialSerive)
	c := make(chan eventsourcing.Event, 5)
	dialService.Subscribe(c)
	dialService.Start()

	dial := &wtf.Dial{
		Name: "test",
	}
	_, ctx0 := MustCreateUser(t, context.Background(), db, &wtf.User{Name: "jane", Email: "jane@gmail.com"})
	err := dialService.CreateDial(ctx0, dial)
	if err != nil {
		t.Fatal(err)
	}
	if dial.ID == 0 {
		t.Fatal("id was not set")
	}
	name := "test2"
	upd := wtf.DialUpdate{Name: &name}
	dial, err = dialService.UpdateDial(ctx0, dial.ID, upd)
	if err != nil {
		t.Fatal(err)
	}
	if dial.Name != name {
		t.Fatal("wrong name after update")
	}
	close(c)

	count := 0
	// loop channel until it's closed
	for e := range c {
		if count == 0 {
			_, ok := e.Data.(*wtf.Created)
			if !ok {
				t.Fatalf("expected Created was %s", e.Reason)
			}
		} else if count == 1 {
			_, ok := e.Data.(*wtf.SelfMembershipCreated)
			if !ok {
				t.Fatalf("expected  SelfMembershipCreated was %s", e.Reason)
			}
		}
		count++
	}
	if count != 3 {
		t.Fatalf("expected 3 events got %d", count)
	}
}
