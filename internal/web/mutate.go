package web

import (
	"log"
	"net/http"
)

// finishMutation answers a mutation consistently: an HTMX request gets the
// affected region swapped into the page, while a plain form post redirects back
// to the referring page so a reload does not resubmit. See design.md - HTMX
// replaces mutated regions, never the whole page.
func (s *Server) finishMutation(w http.ResponseWriter, r *http.Request, render func(http.ResponseWriter)) {
	if isHTMX(r) {
		render(w)
		return
	}
	redirectBack(w, r)
}

// failMutation reports a store failure, keeping the 4xx/5xx split honest.
func (s *Server) failMutation(w http.ResponseWriter, r *http.Request, err error, what string) {
	if status := statusFor(err); status == http.StatusBadRequest || status == http.StatusNotFound {
		s.renderFormError(w, r, err.Error())
		return
	}
	log.Printf("%s: %v", what, err)
	http.Error(w, "database error", http.StatusInternalServerError)
}

// redirectBack sends the browser to the referring page, falling back to /.
func redirectBack(w http.ResponseWriter, r *http.Request) {
	target := "/"
	if ref := r.Referer(); ref != "" {
		target = ref
	}
	http.Redirect(w, r, target, http.StatusSeeOther)
}

// renderFormError reports a rejected mutation to the user. An HTMX request
// swaps the message into the page; a plain form post gets a plain 400 so the
// browser shows it without any client-side state.
func (s *Server) renderFormError(w http.ResponseWriter, r *http.Request, message string) {
	if isHTMX(r) {
		w.WriteHeader(http.StatusBadRequest)
		s.renderRegion(w, "form-error", map[string]any{"Error": message})
		return
	}
	http.Error(w, message, http.StatusBadRequest)
}
