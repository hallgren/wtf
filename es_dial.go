package wtf

import (
	crypto "crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"math/rand"
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
	UserID int

	// Human-readable name of the dial.
	Name string

	// Code used to share the dial with other users.
	// It allows the creation of a shareable link without explicitly inviting users.
	InviteCode string

	// Aggregate WTF level for the dial. This is a computed field based on the
	// average value of each member's WTF level.
	Value int

	// Timestamps for dial creation & last update.
	CreatedAt time.Time
	UpdatedAt time.Time

	// List of associated members and their contributing WTF level.
	// This is only set when returning a single dial.
	Memberships []*DialMembership

	// Deleted indicates if the dial is deleted or not
	Deleted bool
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
	ID     int
	UserID int
	Value  int
}

// MembershipCreated event when a user is adding a dial membership
type MembershipCreated struct {
	ID     int
	UserID int
	Value  int
}

// MembershipValueUpdated event when a user updates her/his dial value
type MembershipUpdated struct {
	ID    int
	Value int
}

// DialValueUpdated event when the aggregated dial value is updated
type DialValueUpdated struct {
	Value int
}

// Renamed update the dial name
type Renamed struct {
	Name string
}

// Deleted indicates that the dial is no more
type Deleted struct{}

// Transition builds the dial entity from its events
func (d *ESDial) Transition(event eventsourcing.Event) {
	dialID, err := strconv.Atoi(event.AggregateID)
	if err != nil {
		panic(err)
	}
	switch e := event.Data.(type) {
	case *Created:
		d.Name = e.Name
		d.CreatedAt = event.Timestamp
		d.UserID = e.OwnerID
		d.InviteCode = e.InviteCode
		d.Memberships = make([]*DialMembership, 0)
	case *SelfMembershipCreated:
		membership := DialMembership{
			ID:        e.ID,
			DialID:    dialID,
			Value:     e.Value,
			UserID:    e.UserID,
			CreatedAt: event.Timestamp,
			UpdatedAt: event.Timestamp,
		}
		d.Memberships = append(d.Memberships, &membership)
	case *MembershipCreated:
		membership := DialMembership{
			ID:        e.ID,
			DialID:    dialID,
			Value:     e.Value,
			UserID:    e.UserID,
			CreatedAt: event.Timestamp,
			UpdatedAt: event.Timestamp,
		}
		d.Memberships = append(d.Memberships, &membership)
	case *MembershipUpdated:
		// find membership
		for _, membership := range d.Memberships {
			if membership.ID == e.ID {
				membership.Value = e.Value
				membership.UpdatedAt = event.Timestamp
			}
		}
	case *Renamed:
		d.Name = e.Name
	case *Deleted:
		d.Deleted = true
	case *DialValueUpdated:
		d.Value = e.Value
	}

	// update the UpdatedAt to the events timestamp
	d.UpdatedAt = event.Timestamp
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
	// Make use of the SetID func to be able to transform a int based id to a string based
	// eventsourcing save its ids as string and the wtf.Dial have ids as int's
	dial.SetID(id())

	// Generate a random invite code.
	inviteCode := make([]byte, 16)
	if _, err := io.ReadFull(crypto.Reader, inviteCode); err != nil {
		return nil, err
	}
	ic := hex.EncodeToString(inviteCode)

	// creates the first event with dial based data
	dial.TrackChange(&dial, &Created{OwnerID: userID, Name: name, InviteCode: ic})

	membershipID, err := strconv.Atoi(id())
	if err != nil {
		return nil, err
	}

	// creates the event with the membership bound to the owner creating the dial
	dial.TrackChange(&dial, &SelfMembershipCreated{ID: membershipID, Value: value, UserID: userID})
	// update the aggregated dial value
	dial.TrackChange(&dial, &DialValueUpdated{Value: value})
	return &dial, nil
}

// Delete removes the dial
func (d *ESDial) Delete(userID int) error {
	if d.Deleted {
		return fmt.Errorf("cant delete an already deleted dial")
	}
	if d.UserID != userID {
		return fmt.Errorf("only the owner can delete the dial")
	}
	d.TrackChange(d, &Deleted{})
	return nil
}

// SetNewName sets new name if not the same
func (d *ESDial) SetNewName(userID int, name string) error {
	if d.Deleted {
		return fmt.Errorf("can't change name on deleted dial")
	}
	if d.UserID != userID {
		return fmt.Errorf("only the owner can change the name")
	}
	if d.Name == name {
		return fmt.Errorf("name is the same")
	}
	d.TrackChange(d, &Renamed{Name: name})
	return nil
}

func (d *ESDial) SetMembershipValue(userID, value int) error {
	// find membership
	for _, membership := range d.Memberships {
		if membership.UserID == userID && membership.Value != value {
			d.TrackChange(d, &MembershipUpdated{ID: membership.ID, Value: value})
			d.TrackChange(d, &DialValueUpdated{Value: d.value()})
		}
	}
	return nil
}

func (d *ESDial) AddMembership(userID int, value int) error {
	if d.Deleted {
		return fmt.Errorf("can't add membership on deleted dial")
	}
	for _, membership := range d.Memberships {
		fmt.Println(membership.UserID, userID)
		if membership.UserID == userID {
			return fmt.Errorf("user membership already exist")
		}
	}
	membershipID, err := strconv.Atoi(id())
	if err != nil {
		return err
	}
	d.TrackChange(d, &MembershipCreated{ID: membershipID, UserID: userID, Value: value})
	d.TrackChange(d, &DialValueUpdated{Value: d.value()})
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

// value returns the current dial value by all memberships
func (d *ESDial) value() int {
	// calculate the dial value from the Memberships
	if len(d.Memberships) > 0 {
		value := 0
		for _, m := range d.Memberships {
			value += m.Value
		}
		return value / len(d.Memberships)
	}
	return 0
}
