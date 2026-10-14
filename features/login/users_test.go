package login

import (
	"testing"
)

func TestCanEditUser(t *testing.T) {
	admin := &SessionUser{Id: 1, IsAdmin: true}
	regular := &SessionUser{Id: 2}

	if !CanEditUser(admin, 2) || !CanEditUser(admin, 1) {
		t.Error("admin should edit anyone")
	}
	if !CanEditUser(regular, 2) {
		t.Error("you should edit yourself")
	}
	if CanEditUser(regular, 1) {
		t.Error("regular user edited someone else")
	}
	if CanEditUser(nil, 1) {
		t.Error("nobody edited someone")
	}
}

func TestCanLoseAdmin(t *testing.T) {
	if CanLoseAdmin(User{Id: 1, IsAdmin: true}, 1) {
		t.Error("removed the last admin")
	}
	if !CanLoseAdmin(User{Id: 1, IsAdmin: true}, 2) {
		t.Error("should be fine with another admin around")
	}
	if !CanLoseAdmin(User{Id: 2}, 1) {
		t.Error("removing a regular user has nothing to do with admins")
	}
}
