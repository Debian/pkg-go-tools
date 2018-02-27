package main

import (
	"net/http"
	"path"
	"strings"

	"salsa.debian.org/go-team/ci/config"
)

// configRepo configures the specified repo underneath go-team/packages with
// go-team-wide settings (CI, webhooks, etc.).
func configRepo(w http.ResponseWriter, r *http.Request) error {
	if r.Method != "POST" {
		http.Error(w, "this URL requires HTTP POST", http.StatusMethodNotAllowed)
		return nil
	}

	// TODO: move the repo extraction into common middleware
	repo := r.FormValue("repo")
	if repo == "" {
		http.Error(w, `no "repo" parameter found`, http.StatusBadRequest)
		return nil
	}
	repo = path.Clean(repo)
	if strings.Contains(repo, "/") {
		http.Error(w, `repo must not contain slashes`, http.StatusBadRequest)
		return nil
	}
	repo = path.Join(group, repo)

	// TODO: log repo names in an auditlog

	<-rateLimit

	p, _, err := salsa.Projects.GetProject(repo)
	if err != nil {
		return err
	}

	return config.All(p)
}
