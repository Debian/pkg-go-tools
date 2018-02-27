package main

import (
	"fmt"
	"net/http"
	"path"
	"strings"

	"salsa.debian.org/go-team/ci/config"

	gitlab "github.com/xanzy/go-gitlab"
)

// createRepo creates a new repository underneath go-team/packages on
// salsa.debian.org.
func createRepo(w http.ResponseWriter, r *http.Request) error {
	if !*repoCreation {
		http.Error(w, "repository creation is disabled by the administrator; please see the mailing list", http.StatusForbidden)
		return nil
	}

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
	name := repo
	repo = path.Join(group, repo)

	// TODO: log repo names in an auditlog

	<-rateLimit

	options := &gitlab.CreateProjectOptions{
		Path:        gitlab.String(name),
		NamespaceID: gitlab.Int(2638),
		Description: gitlab.String(fmt.Sprintf("Debian packaging for %s", name)),
		Visibility:  gitlab.Visibility(gitlab.PublicVisibility),
	}
	p, _, err := salsa.Projects.CreateProject(options)
	if err != nil {
		return fmt.Errorf("CreateProject(%q): %v", *options.Path, err)
	}

	return config.All(p)
}
