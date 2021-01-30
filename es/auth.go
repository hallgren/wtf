package es

import (
	"context"

	"github.com/benbjohnson/wtf"
	"github.com/benbjohnson/wtf/sqlite"
)

var _ wtf.AuthService = (*AuthService)(nil)

type AuthService struct {
	as *sqlite.AuthService
}

func NewAuthService(as *sqlite.AuthService) *AuthService {
	return &AuthService{as: as}
}

func (s *AuthService) FindAuthByID(ctx context.Context, id int) (*wtf.Auth, error) {
	return s.as.FindAuthByID(ctx, id)
}

func (s *AuthService) FindAuths(ctx context.Context, filter wtf.AuthFilter) ([]*wtf.Auth, int, error) {
	return s.as.FindAuths(ctx, filter)
}

func (s *AuthService) CreateAuth(ctx context.Context, auth *wtf.Auth) error {
	return s.as.CreateAuth(ctx, auth)
}

func (s *AuthService) DeleteAuth(ctx context.Context, id int) error {
	return s.as.DeleteAuth(ctx, id)
}
