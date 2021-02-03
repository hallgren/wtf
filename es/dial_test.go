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
	MustSetIDFunc()
	m := memory.Create()
	defer m.Close()
	db := MustOpenDB(t)

	repo := eventsourcing.NewRepository(m, nil)
	sqlDialSerive := sqlite.NewDialService(db)
	dialService := es.NewDialService(repo, sqlDialSerive)
	c := make(chan eventsourcing.Event)
	dialService.Subscribe(c)

	go func() {
		dial := wtf.Dial{
			Name: "test",
		}
		err := dialService.CreateDial(context.Background(), &dial)
		if err != nil {
			t.Fatal(err)
		}
		if dial.ID == 0 {
			t.Fatal("id was not set")
		}
		close(c)
	}()

	count := 0
	// loop channel until its closed
	for range c {
		count++
	}
	if count != 2 {
		t.Fatalf("expected 2 events got %d", count)
	}
}
