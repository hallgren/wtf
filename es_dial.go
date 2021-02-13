package wtf

import (
	crypto "crypto/rand"
	"math/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"strconv"
	"time"

	"github.com/hallgren/eventsourcing"
)

// ESDial represents an aggregate WTF level. They are used to roll up the WTF
// levels of multiple members and show an average WTF level.
//
// A dial is created by a user and can only be edited & deleted by the user who
// created it. Members can be added by sharing an invite link and accepting the
// invitation.
//
// The WTF level for the dial will immediately change when a member's WTF level
// changes and the change will be announced to all other members in real-time.
//
// See the EventService for more information about notifications.
type ESDial struct {
	// include the eventsourcing.AggregateRoot to enable to handle events to state translate the dial entity
	eventsourcing.AggregateRoot

	// Owner of the dial. Only the owner may delete the dial.
	UserID int `json:"userID"`

	// Human-readable name of the dial.
	Name string `json:"name"`

	// Code used to share the dial with other users.
	// It allows the creation of a shareable link without explicitly inviting users.
	InviteCode string `json:"inviteCode,omitempty"`

	// Aggregate WTF level for the dial. This is a computed field based on the
	// average value of each member's WTF level.
	Value int `json:"value"`

	// Timestamps for dial creation & last update.
	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt"`

	// List of associated members and their contributing WTF level.
	// This is only set when returning a single dial.
	Memberships []*DialMembership `json:"memberships,omitempty"`
}

func (d *ESDial) Convert(id int) *Dial {
	return &Dial{
		ID:          id,
		Name:        d.Name,
		UserID:      d.UserID,
		InviteCode:  d.InviteCode,
		CreatedAt:   d.CreatedAt,
		UpdatedAt:   d.UpdatedAt,
		Value:       d.Value,
		Memberships: d.Memberships,
	}
}

// Created event happens when the dial is first created
type Created struct {
	OwnerID    int
	Name       string
	InviteCode string
}

// SelfMembershipCreated event is attached when the dial is created
type SelfMembershipCreated struct {
	ID    int
	UserID int
	Value int
}

// MembershipCreated event when a user is adding a dial membership
type MembershipCreated struct {
	ID     int
	UserID int
	Value  int
}

// Transition builds the dial entity from its events
func (d *ESDial) Transition(event eventsourcing.Event) {
	switch e := event.Data.(type) {
	case *Created:
		d.Name = e.Name
		d.CreatedAt = event.Timestamp
		d.UserID = e.OwnerID
		d.InviteCode = e.InviteCode
		d.Memberships = make([]*DialMembership, 0)
		d.UpdatedAt = event.Timestamp

	case *SelfMembershipCreated:
		dialID, err := strconv.Atoi(event.AggregateID)
		if err != nil {
			panic(err)
		}
		membership := DialMembership{
			ID: e.ID,
			DialID:  dialID,
			Value:     e.Value,
			UserID:    e.UserID,
			CreatedAt: event.Timestamp,
			UpdatedAt: event.Timestamp,
		}
		d.Memberships = append(d.Memberships, &membership)
		d.UpdatedAt = event.Timestamp

	case *MembershipCreated:
		membership := DialMembership{
			ID:        e.ID,
			Value:     e.Value,
			UserID:    e.UserID,
			CreatedAt: event.Timestamp,
			UpdatedAt: event.Timestamp,
		}
		d.Memberships = append(d.Memberships, &membership)
		d.UpdatedAt = event.Timestamp
	}

	// calculate the dial value from the Memberships after the dial entity is built from all events
	// this is calculated on every event but the final event will be the final result of the Value on the dial
	if len(d.Memberships) > 0 {
		value := 0
		for _, m := range d.Memberships {
			value += m.Value
		}
		d.Value = value / len(d.Memberships)
	}
}

func id() string {
	rand.Seed(time.Now().UnixNano())
	return strconv.Itoa(rand.Intn(100000))
}

func NewDial(userID, value int, name string) (*ESDial, error) {
	if name == "" {
		return nil, errors.New("name can't be empty")
	}
	dial := ESDial{}
	dial.SetID(id())

	// Generate a random invite code.
	inviteCode := make([]byte, 16)
	if _, err := io.ReadFull(crypto.Reader, inviteCode); err != nil {
		return nil, err
	}

	ic := hex.EncodeToString(inviteCode)
	dial.TrackChange(&dial, &Created{OwnerID: userID, Name: name, InviteCode: ic})

	membershipID,err := strconv.Atoi(id())
	if err != nil {
		return nil, err
	}

	dial.TrackChange(&dial, &SelfMembershipCreated{ID: membershipID, Value: value, UserID: userID})
	return &dial, nil
}

func (d *ESDial) AddMembership(userID int, value int) error {
	for _, membership := range d.Memberships {
		fmt.Println(membership.UserID, userID)
		if membership.UserID == userID {
			return fmt.Errorf("user membership already exist")
		}
	}
	d.TrackChange(d, &MembershipCreated{UserID: userID, Value: value})
	return nil
}

// MembershipByUserID returns the membership attached to the dial for a given user.
// Returns nil if user is not associated with the dial or if memberships is unset.
func (d *ESDial) MembershipByUserID(userID int) *DialMembership {
	for _, m := range d.Memberships {
		if m.UserID == userID {
			return m
		}
	}
	return nil
}
