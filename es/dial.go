package es

import (
	"context"
	"fmt"
	"time"

	"github.com/benbjohnson/wtf"
	"github.com/benbjohnson/wtf/sqlite"
	"github.com/hallgren/eventsourcing"
)

var _ wtf.DialService = (*DialService)(nil)

type DialService struct {
	UpdateDialFn             func(ctx context.Context, id int, upd wtf.DialUpdate) (*wtf.Dial, error)
	DeleteDialFn             func(ctx context.Context, id int) error
	SetDialMembershipValueFn func(ctx context.Context, dialID, value int) error
	s                        *sqlite.DialService
	repo                     *eventsourcing.Repository
}

func NewDialService(repo *eventsourcing.Repository, s *sqlite.DialService) *DialService {
	return &DialService{repo: repo, s: s}
}

func (s *DialService) Subscribe() {
	subscription := s.repo.SubscriberAll(func(e eventsourcing.Event) {
		fmt.Println(e)
	})
	subscription.Subscribe()

}

func (s *DialService) CreateDial(ctx context.Context, dial *wtf.Dial) error {
	userID := wtf.UserIDFromContext(ctx)
	d, err := wtf.NewDial(userID, dial.Value, dial.Name)
	if err != nil {
		return err
	}
	return s.repo.Save(d)
}

func (s *DialService) FindDialByID(ctx context.Context, id int) (*wtf.Dial, error) {
	return s.s.FindDialByID(ctx, id)
}

func (s *DialService) FindDials(ctx context.Context, filter wtf.DialFilter) ([]*wtf.Dial, int, error) {
	return s.s.FindDials(ctx, filter)
}

func (s *DialService) UpdateDial(ctx context.Context, id int, upd wtf.DialUpdate) (*wtf.Dial, error) {
	return s.UpdateDialFn(ctx, id, upd)
}

func (s *DialService) DeleteDial(ctx context.Context, id int) error {
	return s.DeleteDialFn(ctx, id)
}

func (s *DialService) SetDialMembershipValue(ctx context.Context, dialID, value int) error {
	return s.SetDialMembershipValueFn(ctx, dialID, value)
}

func (s *DialService) AverageDialValueReport(ctx context.Context, start, end time.Time, interval time.Duration) (*wtf.DialValueReport, error) {
	return s.s.AverageDialValueReport(ctx, start, end, interval)
}
