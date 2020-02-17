package api

import (
	"fmt"
	"github.com/xanzy/go-gitlab"
	"path"
	"salsa.debian.org/go-team/ci/config"
)

const (
	group   = "go-team/packages"
	groupId = 2638
)

// Used to store app configuration & information
type AppContext struct {
	// Gitlab client used to create repository on Salsa
	salsaClient *gitlab.Client
	// Whether repo creation is enabled. Can be flipped quickly to limit abuse, should it happen.
	repoCreation bool
}

func NewAppContext(salsaClient *gitlab.Client, repoCreation bool) *AppContext {
	return &AppContext{
		salsaClient:  salsaClient,
		repoCreation: repoCreation,
	}
}

// Get go gitlab project using given name
func (ac *AppContext) GetGoProject(repo string) (*gitlab.Project, error) {
	// Append the namespace
	repo = path.Join(group, repo)
	p, _, err := ac.salsaClient.Projects.GetProject(repo, nil, nil)
	return p, err
}

// Create a new go gitlab project under the golang package team
// with pre-configured settings
func (ac *AppContext) CreateGoProject(repo string) (*gitlab.Project, error) {
	opts := &gitlab.CreateProjectOptions{
		Path:        gitlab.String(repo),
		NamespaceID: gitlab.Int(groupId),
		Description: gitlab.String(fmt.Sprintf("Debian packaging for %s", repo)),
		Visibility:  gitlab.Visibility(gitlab.PublicVisibility),
	}

	// Create the project on Gitlab
	p, _, err := ac.salsaClient.Projects.CreateProject(opts)
	if err != nil {
		return nil, err
	}

	// Configure the project
	if err := config.All(p); err != nil {
		return nil, err
	}

	return p, nil
}
