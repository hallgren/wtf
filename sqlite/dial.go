package sqlite

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"fmt"
	"io"
	"strconv"
	"strings"
	"time"

	"github.com/hallgren/eventsourcing"

	"github.com/benbjohnson/wtf"
)

// DialService represents a service for managing dials.
type DialService struct {
	db *DB
}

// NewDialService returns a new instance of DialService.
func NewDialService(db *DB) *DialService {
	return &DialService{db: db}
}

// FindDialByID retrieves a single dial by ID along with associated memberships.
// Only the dial owner & members can see a dial. Returns ENOTFOUND if dial does
// not exist or user does not have permission to view it.
func (s *DialService) FindDialByID(ctx context.Context, id int) (*wtf.Dial, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	// Fetch dial object and attach owner user.
	dial, err := findDialByID(ctx, tx, id)
	if err != nil {
		return nil, err
	} else if err := attachDialAssociations(ctx, tx, dial); err != nil {
		return nil, err
	}

	return dial, nil
}

// FindDials retrieves a list of dials based on a filter. Only returns dials
// that the user owns or is a member of.
//
// Also returns a count of total matching dials which may different from the
// number of returned dials if the  "Limit" field is set.
func (s *DialService) FindDials(ctx context.Context, filter wtf.DialFilter) ([]*wtf.Dial, int, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, 0, err
	}
	defer tx.Rollback()

	// Fetch list of matching dial objects.
	dials, n, err := findDials(ctx, tx, filter)
	if err != nil {
		return dials, n, err
	}

	// Iterate over dials and attach associated owner user.
	// This should be batched up if using a remote database server.
	for _, dial := range dials {
		if err := attachDialAssociations(ctx, tx, dial); err != nil {
			return dials, n, err
		}
	}
	return dials, n, nil
}

func (s *DialService) CreateDialFromEvent(ctx context.Context, event eventsourcing.Event) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	createDialFromEvent(ctx, tx, event)
	return tx.Commit()
}

func (s *DialService) CreateSelfMembershipFromEvent(ctx context.Context, event eventsourcing.Event) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	createSelfMembershipFromEvent(ctx, tx, event)
	return tx.Commit()
}

func (s *DialService) CreateMembershipFromEvent(ctx context.Context, event eventsourcing.Event) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	createMembershipFromEvent(ctx, tx, event)
	return tx.Commit()
}

func (s *DialService) UpdateDialFromEvent(ctx context.Context, event eventsourcing.Event) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	UpdateNameFromEvent(ctx, tx, event)
	return tx.Commit()
}

func (s *DialService) DeletedDial(ctx context.Context, event eventsourcing.Event) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	DeletedFromEvent(ctx, tx, event)
	return tx.Commit()
}

func (s *DialService) MembershipUpdated(ctx context.Context, event eventsourcing.Event) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	UpdateMembershipEvent(ctx, tx, event)
	return tx.Commit()
}

// CreateDial creates a new dial and assigns the current user as the owner.
// The owner will automatically be added as a member of the new dial.
func (s *DialService) CreateDial(ctx context.Context, dial *wtf.Dial) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	// Create dial and attach associated owner user.
	if err := createDial(ctx, tx, dial); err != nil {
		return err
	} else if err := attachDialAssociations(ctx, tx, dial); err != nil {
		return err
	}
	return tx.Commit()
}

// UpdateDial updates an existing dial by ID. Only the dial owner can update a dial.
// Returns the new dial state even if there was an error during update.
//
// Returns ENOTFOUND if dial does not exist. Returns EUNAUTHORIZED if user
// is not the dial owner.
func (s *DialService) UpdateDial(ctx context.Context, id int, upd wtf.DialUpdate) (*wtf.Dial, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	// Update the dial object and attach associated user to returned dial.
	dial, err := updateDial(ctx, tx, id, upd)
	if err != nil {
		return dial, err
	} else if err := attachDialAssociations(ctx, tx, dial); err != nil {
		return dial, err
	}
	return dial, tx.Commit()
}

// DeleteDial permanently removes a dial by ID. Only the dial owner may delete
// a dial. Returns ENOTFOUND if dial does not exist. Returns EUNAUTHORIZED if
// user is not the dial owner.
func (s *DialService) DeleteDial(ctx context.Context, id int) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	if err := deleteDial(ctx, tx, id); err != nil {
		return err
	}
	return tx.Commit()
}

// Sets the value of the user's membership in a dial. This works the same
// as calling UpdateDialMembership() although it doesn't require that the
// user know their membership ID. Only the dial ID.
//
// Returns ENOTFOUND if the membership does not exist.
func (s *DialService) SetDialMembershipValue(ctx context.Context, dialID, value int) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	// Fetch current user.
	userID := wtf.UserIDFromContext(ctx)

	// Find user's membership.
	memberships, _, err := findDialMemberships(ctx, tx, wtf.DialMembershipFilter{
		DialID: &dialID,
		UserID: &userID,
	})
	if err != nil {
		return err
	} else if len(memberships) == 0 {
		return wtf.Errorf(wtf.ENOTFOUND, "User is not a member of this dial.")
	}

	// Update value on membership.
	if _, err := updateDialMembership(ctx, tx, memberships[0].ID, wtf.DialMembershipUpdate{Value: &value}); err != nil {
		return err
	}
	return tx.Commit()
}

// DialValues returns a list of all stored historical values for a dial.
// This is only used for testing.
func (s *DialService) DialValues(ctx context.Context, id int) ([]int, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	// Execute query to read all values in order for a dial.
	rows, err := tx.QueryContext(ctx, `
		SELECT value
		FROM dial_values
		WHERE dial_id = ?
		ORDER BY "timestamp"
	`, id)
	if err != nil {
		return nil, FormatError(err)
	}

	// Iterate over rows and append to list of values.
	var a []int
	for rows.Next() {
		var v int
		if err := rows.Scan(&v); err != nil {
			return nil, FormatError(err)
		}
		a = append(a, v)
	}
	if err := rows.Err(); err != nil {
		return nil, FormatError(err)
	}
	return a, nil
}

// AverageDialValueReport returns a report of the average dial value across
// all dials that the user is a member of. Average values are computed
// between start & end time and are slotted into given intervals. The
// minimum interval size is one minute.
func (s *DialService) AverageDialValueReport(ctx context.Context, start, end time.Time, interval time.Duration) (*wtf.DialValueReport, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	// Ensure start/end line up with the interval unit.
	start = start.Truncate(interval).UTC()
	end = end.Truncate(interval).UTC()

	// Compute the number of slots between start & end.
	slotN := int(end.Sub(start) / interval)
	report := &wtf.DialValueReport{
		Records: make([]*wtf.DialValueRecord, slotN),
	}

	// Fetch all dials which user is a member or owner.
	dials, _, err := findDials(ctx, tx, wtf.DialFilter{})
	if err != nil {
		return nil, fmt.Errorf("find dials: %w", err)
	}

	// Iterate over each dial and compute value at each slot.
	valuesSlice := make([][]int, len(dials))
	for i, dial := range dials {
		values, err := findDialValueSlotsBetween(ctx, tx, dial.ID, start, end, interval)
		if err != nil {
			return nil, fmt.Errorf("dial values between: id=%d err=%w", dial.ID, err)
		}
		valuesSlice[i] = values
	}

	// Compute average for each slot.
	for i := 0; i < slotN; i++ {
		var avg int
		if len(dials) != 0 {
			var sum int
			for j := range dials {
				sum += valuesSlice[j][i]
			}
			avg = sum / len(valuesSlice)
		}

		// Append record for avg value at a given time.
		report.Records[i] = &wtf.DialValueRecord{
			Timestamp: start.Add(time.Duration(i) * interval),
			Value:     avg,
		}
	}

	return report, nil
}

// findDialByID is a helper function to retrieve a dial by ID.
// Returns ENOTFOUND if dial doesn't exist.
func findDialByID(ctx context.Context, tx *Tx, id int) (*wtf.Dial, error) {
	dials, _, err := findDials(ctx, tx, wtf.DialFilter{ID: &id})
	if err != nil {
		return nil, err
	} else if len(dials) == 0 {
		return nil, &wtf.Error{Code: wtf.ENOTFOUND, Message: "Dial not found."}
	}
	return dials[0], nil
}

// checkDialExists returns nil if a dial does not exist. Otherwise returns ENOTFOUND.
// This is used to avoid permissions checks when inserting related objects.
//
// Unfortunately, SQLite provides poor FOREIGN KEY error descriptions but
// otherwise we would just use those.
func checkDialExists(ctx context.Context, tx *Tx, id int) error {
	var n int
	if err := tx.QueryRowContext(ctx, `SELECT COUNT(1) FROM dials WHERE id = ?`, id).Scan(&n); err != nil {
		return FormatError(err)
	} else if n == 0 {
		return &wtf.Error{Code: wtf.ENOTFOUND, Message: "Dial not found."}
	}
	return nil
}

// findDials retrieves a list of matching dials. Also returns a total matching
// count which may different from the number of results if filter.Limit is set.
func findDials(ctx context.Context, tx *Tx, filter wtf.DialFilter) (_ []*wtf.Dial, n int, err error) {
	// Build WHERE clause. Each part of the WHERE clause is AND-ed together.
	// Values are appended to an arg list to avoid SQL injection.
	where, args := []string{"1 = 1"}, []interface{}{}
	if v := filter.ID; v != nil {
		where, args = append(where, "id = ?"), append(args, *v)
	}

	// Limit to dials user is a member of unless searching by invite code.
	if v := filter.InviteCode; v != nil {
		where, args = append(where, "invite_code = ?"), append(args, *v)
	} else {
		userID := wtf.UserIDFromContext(ctx)
		where = append(where, `(
			id IN (SELECT dial_id FROM dial_memberships dm WHERE dm.user_id = ?)
		)`)
		args = append(args, userID)
	}

	// Execue query with limiting WHERE clause and LIMIT/OFFSET injected.
	rows, err := tx.QueryContext(ctx, `
		SELECT 
		    id,
		    user_id,
		    name,
		    value,
		    invite_code,
		    created_at,
		    updated_at,
		    COUNT(*) OVER()
		FROM dials
		WHERE `+strings.Join(where, " AND ")+`
		ORDER BY id ASC
		`+FormatLimitOffset(filter.Limit, filter.Offset),
		args...,
	)
	if err != nil {
		return nil, n, FormatError(err)
	}
	defer rows.Close()

	// Iterate over rows and deserialize into Dial objects.
	dials := make([]*wtf.Dial, 0)
	for rows.Next() {
		var dial wtf.Dial
		if rows.Scan(
			&dial.ID,
			&dial.UserID,
			&dial.Name,
			&dial.Value,
			&dial.InviteCode,
			(*NullTime)(&dial.CreatedAt),
			(*NullTime)(&dial.UpdatedAt),
			&n,
		); err != nil {
			return nil, 0, err
		}
		dials = append(dials, &dial)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, err
	}

	return dials, n, nil
}

func createDialFromEvent(ctx context.Context, tx *Tx, event eventsourcing.Event) {
	createdEvent := event.Data.(*wtf.Created)
	fmt.Println("createdEvent", createdEvent)
	id, err := strconv.Atoi(event.AggregateID)
	if err != nil {
		panic(err)
	}

	fmt.Println(id, createdEvent.OwnerID)

	// Insert row into database.
	_, err = tx.ExecContext(ctx, `
		INSERT INTO dials (
		    id,
			user_id,
			name,
			invite_code,
			created_at,
			updated_at
		)
		VALUES (?, ?, ?, ?, ?, ?)
	`,
		id,
		createdEvent.OwnerID,
		createdEvent.Name,
		createdEvent.InviteCode,
		(*NullTime)(&event.Timestamp),
		(*NullTime)(&event.Timestamp),
	)
	if err != nil {
		panic(err)
	}
}

func createSelfMembershipFromEvent(ctx context.Context, tx *Tx, event eventsourcing.Event) {
	createSelfMembershipEvent := event.Data.(*wtf.SelfMembershipCreated)

	dialID, err := strconv.Atoi(event.AggregateID)
	if err != nil {
		panic(err)
	}

	// Insert row into database.
	// Execute query to insert membership.
	_, err = tx.ExecContext(ctx, `
		INSERT INTO dial_memberships (
		    id,
			dial_id,
			user_id,
			value,
			created_at,
			updated_at
		)
		VALUES (?, ?, ?, ?, ?, ?)
	`,
		createSelfMembershipEvent.ID,
		dialID,
		createSelfMembershipEvent.UserID,
		createSelfMembershipEvent.Value,
		(*NullTime)(&event.Timestamp),
		(*NullTime)(&event.Timestamp),
	)
	if err != nil {
		panic(err)
	}
}

func createMembershipFromEvent(ctx context.Context, tx *Tx, event eventsourcing.Event) {
	createdMembershipEvent := event.Data.(*wtf.MembershipCreated)

	dialID, err := strconv.Atoi(event.AggregateID)
	if err != nil {
		panic(err)
	}

	// Insert row into database.
	// Execute query to insert membership.
	_, err = tx.ExecContext(ctx, `
		INSERT INTO dial_memberships (
		    id,
			dial_id,
			user_id,
			value,
			created_at,
			updated_at
		)
		VALUES (?, ?, ?, ?, ?, ?)
	`,
		createdMembershipEvent.ID,
		dialID,
		createdMembershipEvent.UserID,
		createdMembershipEvent.Value,
		(*NullTime)(&event.Timestamp),
		(*NullTime)(&event.Timestamp),
	)
	if err != nil {
		panic(err)
	}
}

func UpdateNameFromEvent(ctx context.Context, tx *Tx, event eventsourcing.Event) {
	setNameEvent := event.Data.(*wtf.SetNewName)

	dialID, err := strconv.Atoi(event.AggregateID)
	if err != nil {
		panic(err)
	}

	// Execute update query.
	if _, err := tx.ExecContext(ctx, `
		UPDATE dials
		SET name = ?,
		    updated_at = ?
		WHERE id = ?
	`,
		setNameEvent.Name,
		(*NullTime)(&event.Timestamp),
		dialID,
	); err != nil {
		panic(err)
	}
}

func DeletedFromEvent(ctx context.Context, tx *Tx, event eventsourcing.Event) {
	dialID, err := strconv.Atoi(event.AggregateID)
	if err != nil {
		panic(err)
	}
	// Remove row from database.
	if _, err := tx.ExecContext(ctx, `DELETE FROM dials WHERE id = ?`, dialID); err != nil {
		panic(err)
	}
}

func UpdateMembershipEvent(ctx context.Context, tx *Tx, event eventsourcing.Event) {
	updateMembershipEvent := event.Data.(*wtf.MembershipUpdated)

	if _, err := tx.ExecContext(ctx, `
		UPDATE dial_memberships
		SET value = ?,
		    updated_at = ?
		WHERE id = ?
	`,
		updateMembershipEvent.Value,
		(*NullTime)(&event.Timestamp),
		updateMembershipEvent.ID,
	); err != nil {
		panic(err)
	}
}

// createDial creates a new dial.
func createDial(ctx context.Context, tx *Tx, dial *wtf.Dial) error {
	// Assign dial to the current user.
	// Return an error if the user is not currently logged in.
	userID := wtf.UserIDFromContext(ctx)
	if userID == 0 {
		return wtf.Errorf(wtf.EUNAUTHORIZED, "You must be logged in to create a dial.")
	}
	dial.UserID = wtf.UserIDFromContext(ctx)

	// Generate a random invite code.
	inviteCode := make([]byte, 16)
	if _, err := io.ReadFull(rand.Reader, inviteCode); err != nil {
		return err
	}
	dial.InviteCode = hex.EncodeToString(inviteCode)

	// Set timestamps to current time.
	dial.CreatedAt = tx.now
	dial.UpdatedAt = dial.CreatedAt

	// Perform basic field validation & ensure user exists.
	if err := dial.Validate(); err != nil {
		return err
	} else if _, err := findUserByID(ctx, tx, dial.UserID); err != nil {
		return err
	}

	// Insert row into database.
	result, err := tx.ExecContext(ctx, `
		INSERT INTO dials (
			user_id,
			name,
			invite_code,
			created_at,
			updated_at
		)
		VALUES (?, ?, ?, ?, ?)
	`,
		dial.UserID,
		dial.Name,
		dial.InviteCode,
		(*NullTime)(&dial.CreatedAt),
		(*NullTime)(&dial.UpdatedAt),
	)
	if err != nil {
		return FormatError(err)
	}

	// Read back new dial ID into caller argument.
	id, err := result.LastInsertId()
	if err != nil {
		return err
	}
	dial.ID = int(id)

	// Record initial value to history table.
	if err := insertDialValue(ctx, tx, dial.ID, dial.Value, dial.CreatedAt); err != nil {
		return fmt.Errorf("insert initial value: %w", err)
	}

	// Create self membership automatically.
	if err := createDialMembership(ctx, tx, &wtf.DialMembership{
		DialID: dial.ID,
		UserID: dial.UserID,
	}); err != nil {
		return fmt.Errorf("create self-membership: %w", err)
	}

	return nil
}

// updateDial updates a dial by ID. Returns the new state of the dial after update.
func updateDial(ctx context.Context, tx *Tx, id int, upd wtf.DialUpdate) (*wtf.Dial, error) {
	// Fetch current object state. Return an error if current user is not owner.
	dial, err := findDialByID(ctx, tx, id)
	if err != nil {
		return dial, err
	} else if !wtf.CanEditDial(ctx, dial) {
		return dial, wtf.Errorf(wtf.EUNAUTHORIZED, "You must be the owner can edit a dial.")
	}

	// Update fields, if set.
	if v := upd.Name; v != nil {
		dial.Name = *v
	}
	dial.UpdatedAt = tx.now

	// Perform basic field validation.
	if err := dial.Validate(); err != nil {
		return dial, err
	}

	// Execute update query.
	if _, err := tx.ExecContext(ctx, `
		UPDATE dials
		SET name = ?,
		    updated_at = ?
		WHERE id = ?
	`,
		dial.Name,
		(*NullTime)(&dial.UpdatedAt),
		id,
	); err != nil {
		return dial, FormatError(err)
	}

	return dial, nil
}

// deleteDial permanently deletes a dial by ID. Returns EUNAUTHORIZED if user
// does not own the dial.
func deleteDial(ctx context.Context, tx *Tx, id int) error {
	// Verify object exists & the current user is the owner.
	if dial, err := findDialByID(ctx, tx, id); err != nil {
		return err
	} else if !wtf.CanEditDial(ctx, dial) {
		return wtf.Errorf(wtf.EUNAUTHORIZED, "Only the owner can delete a dial.")
	}

	// Remove row from database.
	if _, err := tx.ExecContext(ctx, `DELETE FROM dials WHERE id = ?`, id); err != nil {
		return FormatError(err)
	}
	return nil
}

// refreshDialValue recomputes the WTF level of a dial by ID and saves it in dials.value.
func refreshDialValue(ctx context.Context, tx *Tx, id int) error {
	// Fetch current dial value.
	var oldValue int
	if err := tx.QueryRowContext(ctx, `SELECT value FROM dials WHERE id = ? `, id).Scan(&oldValue); err == sql.ErrNoRows {
		return nil // no dial, skip
	} else if err != nil {
		return FormatError(err)
	}

	// Compute average value from dial memberships.
	var newValue int
	if err := tx.QueryRowContext(ctx, `
		SELECT CAST(ROUND(IFNULL(AVG(value), 0)) AS INTEGER)
		FROM dial_memberships
		WHERE dial_id = ?
	`,
		id,
	).Scan(
		&newValue,
	); err != nil && err != sql.ErrNoRows {
		return FormatError(err)
	}

	// Exit if the value will not change.
	if oldValue == newValue {
		return nil
	}

	// Update value on dial.
	if _, err := tx.ExecContext(ctx, `
		UPDATE dials
		SET value = ?,
		    updated_at = ?
		WHERE id = ?
	`,
		newValue,
		(*NullTime)(&tx.now),
		id,
	); err != nil {
		return FormatError(err)
	}

	// Record historical value into "dial_values" table.
	if err := insertDialValue(ctx, tx, id, newValue, tx.now); err != nil {
		return fmt.Errorf("insert historical value: %w", err)
	}

	// Publish event to notify other members that the value has changed.
	if err := publishDialEvent(ctx, tx, id, wtf.Event{
		Type: wtf.EventTypeDialValueChanged,
		Payload: &wtf.DialValueChangedPayload{
			ID:    id,
			Value: newValue,
		},
	}); err != nil {
		return fmt.Errorf("publish dial event: %w", err)
	}

	return nil
}

// insertDialValue records a dial value at specific point in time.
func insertDialValue(ctx context.Context, tx *Tx, id int, value int, timestamp time.Time) error {
	// Reduce our precision to only one update per minute.
	timestamp = timestamp.Truncate(1 * time.Minute)

	// Insert a new record or update an existing record for the dial at the given timestamp.
	if _, err := tx.ExecContext(ctx, `
		INSERT INTO dial_values (dial_id, "timestamp", value)
		VALUES (?, ?, ?)
		ON CONFLICT (dial_id, "timestamp") DO UPDATE SET value = ?
	`,
		id, (*NullTime)(&timestamp), value, value,
	); err != nil {
		return FormatError(err)
	}
	return nil
}

// findDialValueSlotsBetween returns the value of a dial at given intervals in a time range.
//
// This function is implemented naively so that we build a set of slots, insert
// values when they've changed, and then we backfill the empty slots with the
// previous value.
//
// There's probably a fancier way to do this in SQL but this was pretty easy.
func findDialValueSlotsBetween(ctx context.Context, tx *Tx, id int, start, end time.Time, interval time.Duration) ([]int, error) {
	values := make([]int, end.Sub(start)/interval)
	if len(values) == 0 {
		return values, nil
	}

	// Mark slots empty. We'll fill them in later.
	for i := range values {
		values[i] = -1
	}

	// Determine initial value at start of report time range.
	var value int
	if err := tx.QueryRowContext(ctx, `
		SELECT value
		FROM dial_values
		WHERE dial_id = ?
		  AND "timestamp" <= ?
		ORDER BY "timestamp" DESC
		LIMIT 1
		`,
		id,
		(*NullTime)(&start),
	).Scan(
		&value,
	); err != nil && err != sql.ErrNoRows {
		return nil, err
	}
	values[0] = value

	// Find all values between start & end.
	rows, err := tx.QueryContext(ctx, `
		SELECT value, "timestamp"
		FROM dial_values
		WHERE dial_id = ?
		  AND "timestamp" >= ?
		  AND "timestamp" < ?
		ORDER BY "timestamp" ASC
	`,
		id,
		(*NullTime)(&start),
		(*NullTime)(&end),
	)
	if err != nil {
		return nil, FormatError(err)
	}
	defer rows.Close()

	// Iterate over rows and assign values to slots.
	for rows.Next() {
		var timestamp time.Time
		if rows.Scan(&value, (*NullTime)(&timestamp)); err != nil {
			return nil, err
		}

		i := int(timestamp.Sub(start) / interval)
		values[i] = value
	}
	if err := rows.Err(); err != nil {
		return nil, err
	} else if err := rows.Close(); err != nil {
		return nil, err
	}

	// Iterate over values to fill empty slots.
	var lastValue int
	for i, v := range values {
		if v != -1 {
			lastValue = v
			continue
		}
		values[i] = lastValue
	}

	return values, nil
}

// publishDialEvent publishes event to the dial members.
func publishDialEvent(ctx context.Context, tx *Tx, id int, event wtf.Event) error {
	// Find all users who are members of the dial.
	rows, err := tx.QueryContext(ctx, `SELECT user_id FROM dial_memberships WHERE dial_id = ?`, id)
	if err != nil {
		return FormatError(err)
	}
	defer rows.Close()

	// Iterate over users and publish event.
	for rows.Next() {
		var userID int
		if err := rows.Scan(&userID); err != nil {
			return err
		}
		tx.db.EventService.PublishEvent(userID, event)
	}

	if err := rows.Err(); err != nil {
		return err
	}
	return nil
}

// attachDialAssociations is a helper function to look up and attach the owner user to the dial.
func attachDialAssociations(ctx context.Context, tx *Tx, dial *wtf.Dial) (err error) {
	if dial.User, err = findUserByID(ctx, tx, dial.UserID); err != nil {
		return fmt.Errorf("attach dial user: %w", err)
	}
	return nil
}
