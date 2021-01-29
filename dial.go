package wtf

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/hallgren/eventsourcing"
)

// Dial constants.
const (
	MaxDialNameLen = 100
)

// Dial represents an aggregate WTF level. They are used to roll up the WTF
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
type Dial struct {
	// include the eventsourcing.AggregateRoot to enable to handle events to state translate the dial entity
	eventsourcing.AggregateRoot

	ID int `json:"id"`

	// Owner of the dial. Only the owner may delete the dial.
	UserID int   `json:"userID"`
	User   *User `json:"user"`

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

// Created event happends when the dial is first created
type Created struct {
	ID         int
	OwnerID    int
	Name       string
	InviteCode string
}

// SelfMembershipCreated event is attached when the dial is created
type SelfMembershipCreated struct {
	ID    int
	Value int
}

// MembershipCreated event when a user is adding a dial membership
type MembershipCreated struct {
	ID     int
	UserID int
	Value  int
}

// Transition builds the dial entity from its events
func (d *Dial) Transition(event eventsourcing.Event) {
	switch e := event.Data.(type) {
	case *Created:
		d.ID = e.ID
		d.Name = e.Name
		d.CreatedAt = event.Timestamp
		d.UserID = e.OwnerID
		d.InviteCode = e.InviteCode
		d.Memberships = make([]*DialMembership, 0)
		d.UpdatedAt = event.Timestamp

	case *SelfMembershipCreated:
		membership := DialMembership{ID: e.ID, Value: e.Value, UserID: d.UserID}
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

func NewDial(userID, value int, name string) (*Dial, error) {
	if name == "" {
		return nil, errors.New("name can't be empty")
	}
	dial := Dial{}
	dial.TrackChange(&dial, &Created{ID: 1, OwnerID: userID, Name: name, InviteCode: "123"})
	dial.TrackChange(&dial, &SelfMembershipCreated{ID: 1, Value: value})
	return &dial, nil
}

func (d *Dial) AddMembership(userID int, value int) error {
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
func (d *Dial) MembershipByUserID(userID int) *DialMembership {
	for _, m := range d.Memberships {
		if m.UserID == userID {
			return m
		}
	}
	return nil
}

// Validate returns an error if dial has invalid fields. Only performs basic validation.
func (d *Dial) Validate() error {
	if d.Name == "" {
		return Errorf(EINVALID, "Dial name required.")
	} else if len(d.Name) > MaxDialNameLen {
		return Errorf(EINVALID, "Dial name too long.")
	} else if d.UserID == 0 {
		return Errorf(EINVALID, "Dial creator required.")
	}
	return nil
}

// CanEditDial returns true if the current user can edit the dial.
// Only the dial owner can edit the dial.
func CanEditDial(ctx context.Context, dial *Dial) bool {
	return dial.UserID == UserIDFromContext(ctx)
}

// DialService represents a service for managing dials.
type DialService interface {
	// Retrieves a single dial by ID along with associated memberships. Only
	// the dial owner & members can see a dial. Returns ENOTFOUND if dial does
	// not exist or user does not have permission to view it.
	FindDialByID(ctx context.Context, id int) (*Dial, error)

	// Retrieves a list of dials based on a filter. Only returns dials that
	// the user owns or is a member of. Also returns a count of total matching
	// dials which may different from the number of returned dials if the
	// "Limit" field is set.
	FindDials(ctx context.Context, filter DialFilter) ([]*Dial, int, error)

	// Creates a new dial and assigns the current user as the owner.
	// The owner will automatically be added as a member of the new dial.
	CreateDial(ctx context.Context, dial *Dial) error

	// Updates an existing dial by ID. Only the dial owner can update a dial.
	// Returns the new dial state even if there was an error during update.
	//
	// Returns ENOTFOUND if dial does not exist. Returns EUNAUTHORIZED if user
	// is not the dial owner.
	UpdateDial(ctx context.Context, id int, upd DialUpdate) (*Dial, error)

	// Permanently removes a dial by ID. Only the dial owner may delete a dial.
	// Returns ENOTFOUND if dial does not exist. Returns EUNAUTHORIZED if user
	// is not the dial owner.
	DeleteDial(ctx context.Context, id int) error

	// Sets the value of the user's membership in a dial. This works the same
	// as calling UpdateDialMembership() although it doesn't require that the
	// user know their membership ID. Only the dial ID.
	//
	// Returns ENOTFOUND if the membership does not exist.
	SetDialMembershipValue(ctx context.Context, dialID, value int) error

	// AverageDialValueReport returns a report of the average dial value across
	// all dials that the user is a member of. Average values are computed
	// between start & end time and are slotted into given intervals. The
	// minimum interval size is one minute.
	AverageDialValueReport(ctx context.Context, start, end time.Time, interval time.Duration) (*DialValueReport, error)
}

// DialFilter represents a filter used by FindDials().
type DialFilter struct {
	// Filtering fields.
	ID         *int    `json:"id"`
	InviteCode *string `json:"inviteCode"`

	// Restrict to subset of range.
	Offset int `json:"offset"`
	Limit  int `json:"limit"`
}

// DialUpdate represents a set of fields to update on a dial.
type DialUpdate struct {
	Name *string `json:"name"`
}

// DialValueReport represents a report generated by AverageDialValueReport().
// Each record represents the average value within an interval of time.
type DialValueReport struct {
	Records []*DialValueRecord
}

// DialValueRecord represents an average dial value at a given point in time
// for the DialValueReport.
type DialValueRecord struct {
	Value     int       `json:"value"`
	Timestamp time.Time `json:"timestamp"`
}

// GoString prints a more easily readable representation for debugging.
// The timestamp field is represented as an RFC 3339 string instead of a pointer.
func (r *DialValueRecord) GoString() string {
	return fmt.Sprintf("&wtf.DialValueRecord{Value:%d, Timestamp:%q}", r.Value, r.Timestamp.Format(time.RFC3339))
}
