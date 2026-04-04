package web

import (
	"fmt"
	"net/http"
	"unicode/utf8"

	"github.com/sorin-work/bookshelf/internal/auth"
)

type passwordForm struct {
	Errors formErrors
}

func checkNewPassword(password, confirm string, errs formErrors) {
	switch {
	case utf8.RuneCountInString(password) < auth.MinPasswordLength:
		errs["password"] = "Use at least 8 characters."
	case len(password) > 72:
		errs["password"] = "Use at most 72 characters."
	case password != confirm:
		errs["confirm"] = "The two passwords don't match."
	}
}

func (s *Server) passwordPage(w http.ResponseWriter, r *http.Request) {
	s.render(w, r, http.StatusOK, "password.html", page{Title: "Change password", Data: passwordForm{}})
}

// changePassword lets a signed in user pick a new password. Their other
// sessions are signed out.
func (s *Server) changePassword(w http.ResponseWriter, r *http.Request) {
	u := currentUser(r)
	f := passwordForm{Errors: formErrors{}}
	if !auth.CheckPassword(u.PasswordHash, r.PostFormValue("current")) {
		f.Errors["current"] = "That isn't your current password."
	}
	checkNewPassword(r.PostFormValue("password"), r.PostFormValue("confirm"), f.Errors)
	if f.Errors.any() {
		s.render(w, r, http.StatusUnprocessableEntity, "password.html", page{Title: "Change password", Data: f})
		return
	}
	hash, err := auth.HashPassword(r.PostFormValue("password"))
	if err != nil {
		s.serverError(w, r, err)
		return
	}
	keep := ""
	if c, err := r.Cookie(sessionCookie); err == nil {
		keep = auth.HashToken(c.Value)
	}
	if err := s.store.SetPassword(r.Context(), u.ID, hash, keep); err != nil {
		s.serverError(w, r, err)
		return
	}
	s.log.Info("password changed", "user_id", u.ID)
	s.setFlash(w, "Password changed. You've been signed out on other devices.")
	http.Redirect(w, r, "/account", http.StatusSeeOther)
}

// setMemberPassword lets a librarian give a member (new or forgetful) a
// password at the desk.
func (s *Server) setMemberPassword(w http.ResponseWriter, r *http.Request) {
	m, ok := s.memberFromPath(w, r)
	if !ok {
		return
	}
	errs := formErrors{}
	password := r.PostFormValue("password")
	checkNewPassword(password, password, errs)
	if msg, bad := errs["password"]; bad {
		http.Error(w, msg, http.StatusUnprocessableEntity)
		return
	}
	hash, err := auth.HashPassword(password)
	if err != nil {
		s.serverError(w, r, err)
		return
	}
	if err := s.store.SetPassword(r.Context(), m.ID, hash, ""); err != nil {
		s.serverError(w, r, err)
		return
	}
	s.log.Info("member password set", "member_id", m.ID, "by", currentUser(r).ID)
	s.setFlash(w, "Password set. "+m.Name+" can now log in with "+m.Email+".")
	http.Redirect(w, r, fmt.Sprintf("/members/%d", m.ID), http.StatusSeeOther)
}
