package es

import (
	"context"

	"github.com/benbjohnson/wtf"
	"github.com/benbjohnson/wtf/sqlite"
)

var _ wtf.UserService = (*UserService)(nil)

// UserService represents a service for managing users.
type UserService struct {
	us *sqlite.UserService
}

func NewUserService(us *sqlite.UserService) *UserService {
	return &UserService{us: us}
}

func (s *UserService) FindUserByID(ctx context.Context, id int) (*wtf.User, error) {
	return s.us.FindUserByID(ctx, id)
}

func (s *UserService) FindUsers(ctx context.Context, filter wtf.UserFilter) ([]*wtf.User, int, error) {
	return s.us.FindUsers(ctx, filter)
}

func (s *UserService) CreateUser(ctx context.Context, user *wtf.User) error {
	return s.us.CreateUser(ctx, user)
}

func (s *UserService) UpdateUser(ctx context.Context, id int, upd wtf.UserUpdate) (*wtf.User, error) {
	return s.us.UpdateUser(ctx, id, upd)
}

func (s *UserService) DeleteUser(ctx context.Context, id int) error {
	return s.us.DeleteUser(ctx, id)
}
