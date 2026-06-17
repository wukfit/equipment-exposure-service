package domain

import "github.com/google/uuid"

type User struct {
	id   uuid.UUID
	name string
}

func NewUser(id uuid.UUID, name string) (*User, error) {
	if name == "" {
		return nil, ErrInvalidName
	}
	return &User{id: id, name: name}, nil
}

func (u *User) ID() uuid.UUID { return u.id }
func (u *User) Name() string  { return u.name }
