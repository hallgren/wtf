package wtf_test

import (
	"testing"

	"github.com/benbjohnson/wtf"
)

func TestCreateDial(t *testing.T) {
	dial, err := wtf.NewDial(1, 43, "123")
	if err != nil {
		t.Fatal(err)
	}

	if dial.Value != 43 {
		t.Fatalf("epextec Value 43 got %d", 43)
	}
	if dial.UserID != 1 {
		t.Fatalf("expected userID to be 1 got %d", dial.UserID)
	}
	if dial.Name != "123" {
		t.Fatalf("expected name to be 123 but was %s", dial.Name)
	}
	if dial.CreatedAt.IsZero() {
		t.Fatal("expected CreatedAt to be set")
	}
	if dial.UpdatedAt.IsZero() {
		t.Fatal("expected UpdatedAt to be set")
	}
	if len(dial.Memberships) != 1 {
		t.Fatalf("expected one membership got %d", len(dial.Memberships))
	}
	if dial.InviteCode == "" {
		t.Fatal("expected invite code to be set")
	}
}

func TestAddMembership(t *testing.T) {
	dial, err := wtf.NewDial(1, 43, "123")
	if err != nil {
		t.Fatal(err)
	}

	err = dial.AddMembership(2, 33)
	if err != nil {
		t.Fatal(err)
	}
	if len(dial.Memberships) != 2 {
		t.Fatalf("expected 2 membershipa got %d", len(dial.Memberships))
	}
	if dial.Value != (33+43)/2 {
		t.Fatalf("expected 38 in Value got %v", dial.Value)
	}
}

func TestAddExistingMembership(t *testing.T) {
	dial, err := wtf.NewDial(1, 43, "123")
	if err != nil {
		t.Fatal(err)
	}
	err = dial.AddMembership(1, 33)
	if err == nil {
		t.Fatal("expected error user membership already exist, got none")
	}
}
