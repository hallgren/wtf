package es

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"time"

	"github.com/benbjohnson/wtf"
	"github.com/benbjohnson/wtf/sqlite"
	"github.com/hallgren/eventsourcing"
)

var _ wtf.DialService = (*DialService)(nil)

type DialService struct {
	SetDialMembershipValueFn func(ctx context.Context, dialID, value int) error
	s                        *sqlite.DialService
	repo                     *eventsourcing.Repository
}

func NewDialService(repo *eventsourcing.Repository, s *sqlite.DialService) *DialService {
	return &DialService{repo: repo, s: s}
}

// Subscribe is currently only for testing but will probably fade away
func (s *DialService) Subscribe(c chan eventsourcing.Event) {
	subscription := s.repo.SubscriberAll(func(e eventsourcing.Event) {
		c <- e
	})
	subscription.Subscribe()
}

func (s *DialService) Start() {
	// consume the Event chan and build the read model

	subscription := s.repo.SubscriberSpecificEvent(func(e eventsourcing.Event) {
		// build the read model in the sqlite database
		s.s.CreateDialFromEvent(context.Background(), e)
	}, &wtf.Created{})
	go subscription.Subscribe()

	subscriptionSelfMember := s.repo.SubscriberSpecificEvent(func(e eventsourcing.Event) {
		// build the read model in the sqlite database
		s.s.CreateSelfMembershipFromEvent(context.Background(), e)
	}, &wtf.SelfMembershipCreated{})
	go subscriptionSelfMember.Subscribe()

	subscriptionMembership := s.repo.SubscriberSpecificEvent(func(e eventsourcing.Event) {
		// build the read model in the sqlite database
		s.s.CreateMembershipFromEvent(context.Background(), e)
	}, &wtf.MembershipCreated{})
	go subscriptionMembership.Subscribe()

	subscriptionUpdateDial := s.repo.SubscriberSpecificEvent(func(e eventsourcing.Event) {
		// build the read model in the sqlite database
		s.s.UpdateDialFromEvent(context.Background(), e)
	}, &wtf.SetNewName{})
	go subscriptionUpdateDial.Subscribe()

	subscriptionDeleteDial := s.repo.SubscriberSpecificEvent(func(e eventsourcing.Event) {
		// build the read model in the sqlite database
		s.s.DeletedDial(context.Background(), e)
	}, &wtf.Deleted{})
	go subscriptionDeleteDial.Subscribe()
}

func (s *DialService) CreateDial(ctx context.Context, dial *wtf.Dial) error {
	userID := wtf.UserIDFromContext(ctx)
	d, err := wtf.NewDial(userID, dial.Value, dial.Name)
	if err != nil {
		return err
	}
	id, err := strconv.Atoi(d.AggregateID)
	if err != nil {
		return err
	}
	// set the dial id
	dial.ID = id
	return s.repo.Save(d)
}

func (s *DialService) FindDialByID(ctx context.Context, id int) (*wtf.Dial, error) {
	dial := wtf.ESDial{}
	err := s.repo.Get(fmt.Sprint(id), &dial)
	return dial.Convert(id), err
}

func (s *DialService) FindDials(ctx context.Context, filter wtf.DialFilter) ([]*wtf.Dial, int, error) {
	return s.s.FindDials(ctx, filter)
}

func (s *DialService) UpdateDial(ctx context.Context, id int, upd wtf.DialUpdate) (*wtf.Dial, error) {
	if upd.Name == nil {
		return nil, errors.New("name can't be nil")
	}
	dial := wtf.ESDial{}
	s.repo.Get(fmt.Sprint(id), &dial)
	userID := wtf.UserIDFromContext(ctx)
	dial.SetNewName(userID, *upd.Name)
	s.repo.Save(&dial)
	return dial.Convert(id), nil
}

func (s *DialService) DeleteDial(ctx context.Context, id int) error {
	dial := wtf.ESDial{}
	s.repo.Get(fmt.Sprint(id), &dial)
	userID := wtf.UserIDFromContext(ctx)
	err := dial.Delete(userID)
	if err != nil {
		return err
	}
	return s.repo.Save(&dial)
}

func (s *DialService) SetDialMembershipValue(ctx context.Context, dialID, value int) error {
	return s.SetDialMembershipValueFn(ctx, dialID, value)
}

func (s *DialService) AverageDialValueReport(ctx context.Context, start, end time.Time, interval time.Duration) (*wtf.DialValueReport, error) {
	return s.s.AverageDialValueReport(ctx, start, end, interval)
}
