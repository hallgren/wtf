package es_test

import (
	"context"
	"testing"

	"github.com/benbjohnson/wtf/es"
	"github.com/hallgren/eventsourcing"
	"github.com/hallgren/eventsourcing/eventstore/memory"
)

func TestCreateDial(t *testing.T) {
	m := memory.Create()
	defer m.Close()

	repo := eventsourcing.NewRepository(m, nil)
	dialService := es.NewDialService(repo, nil)
	err := dialService.CreateDial(context.Background(), nil)
	if err != nil {
		t.Fatal(err)
	}
}
