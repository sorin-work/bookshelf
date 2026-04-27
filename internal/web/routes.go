package web

import "net/http"

// Handler returns the router with every route and middleware attached.
func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	s.routes(mux)
	return s.recoverPanics(s.logRequests(securityHeaders(s.csrf(s.loadUser(mux)))))
}

// routes registers every page. Anyone may read the catalogue; members
// need to be logged in for their own pages; the rest is for librarians.
func (s *Server) routes(mux *http.ServeMux) {
	mux.HandleFunc("GET /healthz", s.healthz)
	mux.Handle("GET /static/", http.StripPrefix("/static/", staticHandler()))
	mux.HandleFunc("GET /favicon.ico", func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "/static/favicon.svg", http.StatusMovedPermanently)
	})
	mux.HandleFunc("GET /{$}", s.home)

	mux.HandleFunc("GET /signup", s.signupPage)
	mux.HandleFunc("POST /signup", s.signup)
	mux.HandleFunc("GET /login", s.loginPage)
	mux.HandleFunc("POST /login", s.login)
	mux.HandleFunc("POST /logout", s.logout)

	mux.HandleFunc("GET /account", s.requireUser(s.account))
	mux.HandleFunc("GET /account/password", s.requireUser(s.passwordPage))
	mux.HandleFunc("POST /account/password", s.requireUser(s.changePassword))
	mux.HandleFunc("GET /catalogue", s.catalogue)
	mux.HandleFunc("GET /titles/new", s.requireLibrarian(s.newTitlePage))
	mux.HandleFunc("POST /titles", s.requireLibrarian(s.createTitle))
	mux.HandleFunc("GET /titles/lookup", s.requireLibrarian(s.lookupTitle))
	mux.HandleFunc("GET /titles/import", s.requireLibrarian(s.importPage))
	mux.HandleFunc("POST /titles/import", s.requireLibrarian(s.importTitles))
	mux.HandleFunc("GET /titles/{id}", s.titlePage)
	mux.HandleFunc("GET /titles/{id}/edit", s.requireLibrarian(s.editTitlePage))
	mux.HandleFunc("POST /titles/{id}", s.requireLibrarian(s.updateTitle))
	mux.HandleFunc("POST /titles/{id}/copies", s.requireLibrarian(s.addCopy))
	mux.HandleFunc("POST /titles/{id}/delete", s.requireLibrarian(s.deleteTitle))
	mux.HandleFunc("POST /copies/{id}/status", s.requireLibrarian(s.setCopyStatus))
	mux.HandleFunc("POST /copies/{id}", s.requireLibrarian(s.updateCopy))
	mux.HandleFunc("POST /titles/{id}/hold", s.requireUser(s.placeHold))
	mux.HandleFunc("POST /holds/{id}/cancel", s.requireUser(s.cancelHold))
	mux.HandleFunc("GET /holds", s.requireLibrarian(s.holds))
	mux.HandleFunc("GET /overdue", s.requireLibrarian(s.overdue))
	mux.HandleFunc("GET /overdue.csv", s.requireLibrarian(s.overdueCSV))
	mux.HandleFunc("GET /dashboard", s.requireLibrarian(s.dashboard))

	mux.HandleFunc("GET /desk", s.requireLibrarian(s.desk))
	mux.HandleFunc("GET /desk/member", s.requireLibrarian(s.deskMember))
	mux.HandleFunc("POST /desk/checkout", s.requireLibrarian(s.deskCheckout))
	mux.HandleFunc("POST /desk/return", s.requireLibrarian(s.deskReturn))
	mux.HandleFunc("POST /loans/{id}/renew", s.requireUser(s.renewLoan))

	mux.HandleFunc("GET /settings", s.requireLibrarian(s.settings))
	mux.HandleFunc("POST /settings", s.requireLibrarian(s.saveSettings))

	mux.HandleFunc("GET /members", s.requireLibrarian(s.members))
	mux.HandleFunc("GET /members/new", s.requireLibrarian(s.newMemberPage))
	mux.HandleFunc("POST /members", s.requireLibrarian(s.createMember))
	mux.HandleFunc("GET /members/{id}", s.requireLibrarian(s.memberPage))
	mux.HandleFunc("GET /members/{id}/edit", s.requireLibrarian(s.editMemberPage))
	mux.HandleFunc("POST /members/{id}", s.requireLibrarian(s.updateMember))
	mux.HandleFunc("POST /members/{id}/renew-card", s.requireLibrarian(s.renewCard))
	mux.HandleFunc("POST /members/{id}/role", s.requireLibrarian(s.setRole))
	mux.HandleFunc("GET /members/{id}/slip", s.requireLibrarian(s.slip))
	mux.HandleFunc("POST /members/{id}/active", s.requireLibrarian(s.setActive))
	mux.HandleFunc("POST /members/{id}/password", s.requireLibrarian(s.setMemberPassword))
	mux.HandleFunc("POST /members/{id}/charges", s.requireLibrarian(s.addCharge))
	mux.HandleFunc("POST /fines/{id}/resolve", s.requireLibrarian(s.resolveFine))
}
