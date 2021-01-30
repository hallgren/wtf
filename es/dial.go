package es

import (
	"context"
	"fmt"
	"time"

	"github.com/benbjohnson/wtf"
	"github.com/hallgren/eventsourcing"
)

var _ wtf.DialService = (*DialService)(nil)

type DialService struct {
	repo *eventsourcing.Repository
}

func NewDialService(repo *eventsourcing.Repository) *DialService {
	return &DialService{repo: repo}
}

func (s *DialService) FindDialByID(ctx context.Context, id int) (*wtf.Dial, error) {
	d := wtf.Dial{}
	err := s.repo.Get(fmt.Sprintf("%v", id), &d)
	if err != nil {
		return &d, err
	}
	// make sure the owner of the dial is the same as the user id from ctx
	return &d, nil
}

func (s *DialService) FindDials(ctx context.Context, filter wtf.DialFilter) ([]*wtf.Dial, int, error) {
	return s.FindDials(ctx, filter)
}

func (s *DialService) CreateDial(ctx context.Context, dialCreate *wtf.DialCreate) (*wtf.Dial, error) {
	userID := wtf.UserIDFromContext(ctx)
	dial, err := wtf.NewDial(dialCreate.ID, userID, dialCreate.Value, dialCreate.Name)
	if err != nil {
		return nil, err
	}
	err = s.repo.Save(dial)
	return dial, err
}

func (s *DialService) UpdateDial(ctx context.Context, id int, upd wtf.DialUpdate) (*wtf.Dial, error) {
	return nil, nil
}

func (s *DialService) DeleteDial(ctx context.Context, id int) error {
	return nil
}

func (s *DialService) SetDialMembershipValue(ctx context.Context, dialID, value int) error {
	return nil
}

func (s *DialService) AverageDialValueReport(ctx context.Context, start, end time.Time, interval time.Duration) (*wtf.DialValueReport, error) {
	return nil, nil
}
