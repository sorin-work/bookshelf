package web

import (
	"net/http"
	"testing"
)

func TestChangePassword(t *testing.T) {
	env := newTestEnv(t)
	phone := env.client()
	phone.signup("Ana", "ana@example.com", "long enough")
	laptop := env.client()
	wantRedirect(t, laptop.login("ana@example.com", "long enough"), "/")

	tests := []struct {
		current, password, confirm, want string
	}{
		{"wrong", "brand new pass", "brand new pass", "That isn&#39;t your current password."},
		{"long enough", "short", "short", "Use at least 8 characters."},
		{"long enough", "brand new pass", "brand new pas", "The two passwords don&#39;t match."},
	}
	for _, tt := range tests {
		r := laptop.post("/account/password", titleValues("current", tt.current, "password", tt.password, "confirm", tt.confirm))
		wantStatus(t, r, http.StatusUnprocessableEntity)
		wantBody(t, r, tt.want)
	}

	r := laptop.post("/account/password", titleValues("current", "long enough", "password", "brand new pass", "confirm", "brand new pass"))
	wantRedirect(t, r, "/account")
	wantBody(t, laptop.get("/account"), "Password changed.")
	// The other session is gone.
	wantRedirect(t, phone.get("/account"), "/login?next=%2Faccount")

	wantStatus(t, env.client().login("ana@example.com", "long enough"), http.StatusUnauthorized)
	wantRedirect(t, env.client().login("ana@example.com", "brand new pass"), "/")
}

func TestLibrarianSetsMemberPassword(t *testing.T) {
	env := newTestEnv(t)
	lib := env.librarian()
	r := lib.post("/members", titleValues("name", "Desk Person", "email", "desk@example.com"))
	wantStatus(t, r, http.StatusSeeOther)
	wantBody(t, lib.get(r.location), "Set a password for online access")

	wantStatus(t, lib.post(r.location+"/password", titleValues("password", "short")), http.StatusUnprocessableEntity)
	wantRedirect(t, lib.post(r.location+"/password", titleValues("password", "given at the desk")), r.location)
	wantRedirect(t, env.client().login("desk@example.com", "given at the desk"), "/")
}
