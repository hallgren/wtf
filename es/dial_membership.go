package es

import (
	"context"

	"github.com/benbjohnson/wtf"
	"github.com/benbjohnson/wtf/sqlite"
)

var _ wtf.DialMembershipService = (*DialMembershipService)(nil)

// DialMembershipService represents a service for managing dial memberships in SQLite.
type DialMembershipService struct {
	dms *sqlite.DialMembershipService
}

// NewDialMembershipService returns a new instance of DialMembershipService.
func NewDialMembershipService(dms *sqlite.DialMembershipService) *DialMembershipService {
	return &DialMembershipService{dms: dms}
}

func (s *DialMembershipService) FindDialMembershipByID(ctx context.Context, id int) (*wtf.DialMembership, error) {
	return s.dms.FindDialMembershipByID(ctx, id)
}

func (s *DialMembershipService) FindDialMemberships(ctx context.Context, filter wtf.DialMembershipFilter) ([]*wtf.DialMembership, int, error) {
	return s.dms.FindDialMemberships(ctx, filter)
}

// CreateDialMembership should create a membership on the dial
func (s *DialMembershipService) CreateDialMembership(ctx context.Context, membership *wtf.DialMembership) error {
	return s.dms.CreateDialMembership(ctx, membership)
}

func (s *DialMembershipService) UpdateDialMembership(ctx context.Context, id int, upd wtf.DialMembershipUpdate) (*wtf.DialMembership, error) {
	return s.dms.UpdateDialMembership(ctx, id, upd)
}

func (s *DialMembershipService) DeleteDialMembership(ctx context.Context, id int) error {
	return s.dms.DeleteDialMembership(ctx, id)
}
