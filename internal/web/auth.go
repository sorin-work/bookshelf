package web

import (
	"errors"
	"net/http"
	"strings"
	"unicode/utf8"

	"github.com/sorin-work/bookshelf/internal/auth"
	"github.com/sorin-work/bookshelf/internal/store"
)

type authForm struct {
	Name   string
	Email  string
	Next   string
	Errors formErrors
}

func (s *Server) signupPage(w http.ResponseWriter, r *http.Request) {
	if currentUser(r) != nil {
		http.Redirect(w, r, "/", http.StatusSeeOther)
		return
	}
	s.render(w, r, http.StatusOK, "signup.html", page{Title: "Sign up", Data: authForm{}})
}

func (s *Server) signup(w http.ResponseWriter, r *http.Request) {
	f := authForm{
		Name:   strings.TrimSpace(r.PostFormValue("name")),
		Email:  strings.TrimSpace(r.PostFormValue("email")),
		Errors: formErrors{},
	}
	password := r.PostFormValue("password")

	if f.Name == "" {
		f.Errors["name"] = "Tell us your name."
	} else if utf8.RuneCountInString(f.Name) > 100 {
		f.Errors["name"] = "Keep it under 100 characters."
	}
	if !validEmail(f.Email) {
		f.Errors["email"] = "Enter a valid email address."
	}
	if utf8.RuneCountInString(password) < auth.MinPasswordLength {
		f.Errors["password"] = "Use at least 8 characters."
	} else if len(password) > 72 {
		f.Errors["password"] = "Use at most 72 characters."
	}
	if f.Errors.any() {
		s.render(w, r, http.StatusUnprocessableEntity, "signup.html", page{Title: "Sign up", Data: f})
		return
	}

	hash, err := auth.HashPassword(password)
	if err != nil {
		s.serverError(w, r, err)
		return
	}
	u, err := s.store.CreateUser(r.Context(), store.NewUser{Email: f.Email, Name: f.Name, PasswordHash: hash}, s.now())
	if errors.Is(err, store.ErrConflict) {
		f.Errors["email"] = "There is already an account with this email. Log in instead?"
		s.render(w, r, http.StatusUnprocessableEntity, "signup.html", page{Title: "Sign up", Data: f})
		return
	}
	if err != nil {
		s.serverError(w, r, err)
		return
	}
	if err := s.startSession(w, r, u.ID); err != nil {
		s.serverError(w, r, err)
		return
	}
	s.log.Info("user signed up", "user_id", u.ID)
	http.Redirect(w, r, "/", http.StatusSeeOther)
}

func (s *Server) loginPage(w http.ResponseWriter, r *http.Request) {
	if currentUser(r) != nil {
		http.Redirect(w, r, "/", http.StatusSeeOther)
		return
	}
	s.render(w, r, http.StatusOK, "login.html", page{Title: "Log in", Data: authForm{Next: r.URL.Query().Get("next")}})
}

func (s *Server) login(w http.ResponseWriter, r *http.Request) {
	f := authForm{
		Email:  strings.TrimSpace(r.PostFormValue("email")),
		Next:   r.PostFormValue("next"),
		Errors: formErrors{},
	}
	password := r.PostFormValue("password")

	u, err := s.store.UserByEmail(r.Context(), f.Email)
	if err != nil && !errors.Is(err, store.ErrNotFound) {
		s.serverError(w, r, err)
		return
	}
	if err != nil || !auth.CheckPassword(u.PasswordHash, password) {
		f.Errors["form"] = "That email and password don't match an account."
		s.render(w, r, http.StatusUnauthorized, "login.html", page{Title: "Log in", Data: f})
		return
	}
	if err := s.startSession(w, r, u.ID); err != nil {
		s.serverError(w, r, err)
		return
	}
	http.Redirect(w, r, safeNext(f.Next), http.StatusSeeOther)
}
