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
