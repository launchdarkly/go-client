package roles

import "fmt"

type RoleType string

const (
	ReaderRole  RoleType = "reader"
	WriterRole  RoleType = "writer"
	AdminRole   RoleType = "admin"
	OwnerRole   RoleType = "owner"
	InvalidRole RoleType = "invalid"
)

var ValidRoles = []RoleType{ReaderRole, WriterRole, AdminRole, OwnerRole}

func (r RoleType) String() string {
	return string(r)
}

func IsValidRole(role string) bool {
	for _, r := range ValidRoles {
		if r.String() == role {
			return true
		}
	}
	return false
}

func MakeRole(role string) (RoleType, error) {
	for _, r := range ValidRoles {
		if r.String() == role {
			return r, nil
		}
	}
	return InvalidRole, fmt.Errorf("Invalid role: %s", role)
}
