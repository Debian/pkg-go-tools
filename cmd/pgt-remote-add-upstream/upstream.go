// Binary pgt-remote-add-upstream configures an upstream git remote based on the
// X-Vcs-Upstream-Git value in debian/control, or XS-Go-Import-Path, or a line
// matching “export DH_GOPKG := (.*)” in debian/rules.
//
// pgt-remote-add-upstream is intended to be used as a gbp postclone hook.
//
// This binary will be obsolete once git-buildpackage implements the feature
// itself: https://bugs.debian.org/888313
package main

import (
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"

	"golang.org/x/tools/go/vcs"

	"pault.ag/go/debian/control"
)

func resolveImportPath(importPath string) (string, error) {
	rr, err := vcs.RepoRootForImportPath(importPath, false)
	if err != nil {
		return "", err
	}
	if rr.VCS.Name != "Git" {
		return "", fmt.Errorf("import path %s does not use git", importPath)
	}
	return rr.Repo, nil
}

var gopkgRe = regexp.MustCompile(`^export DH_GOPKG := (.*)$`)

func vcsUpstream(dir string) (string, error) {
	var s control.SourceParagraph
	f, err := os.Open(filepath.Join(dir, "debian/control"))
	if err != nil {
		return "", err
	}
	defer f.Close()
	if err := control.Unmarshal(&s, f); err != nil {
		return "", err
	}

	// Return machine-readable X-Vcs-Upstream-Git if present:
	for key, val := range s.Values {
		if strings.ToLower(key) == "x-vcs-upstream-git" {
			return val, nil
		}
	}

	// Return machine-readable XS-Go-Import-Path if present and if the import
	// path’s VCS is git:
	for key, val := range s.Values {
		if strings.ToLower(key) == "xs-go-import-path" {
			return resolveImportPath(val)
		}
	}

	// Examine debian/rules for DH_GOPKG environment variable:
	b, err := ioutil.ReadFile(filepath.Join(dir, "debian/rules"))
	if err != nil {
		return "", err
	}
	for _, line := range strings.Split(strings.TrimSpace(string(b)), "\n") {
		matches := gopkgRe.FindStringSubmatch(line)
		if matches == nil {
			continue
		}
		return resolveImportPath(matches[1])
	}

	return "", fmt.Errorf("could not find VCS")
}

func logic() error {
	dir, err := os.Getwd()
	if err != nil {
		return err
	}
	vcs, err := vcsUpstream(dir)
	if err != nil {
		return err
	}

	log.Printf("Configuring upstream git remote %s", vcs)

	if err := exec.Command("git", "remote", "add", "upstream", vcs).Run(); err != nil {
		return fmt.Errorf("git remote add upstream %s: %v", vcs, err)
	}

	if err := exec.Command("git", "fetch", "upstream").Run(); err != nil {
		return fmt.Errorf("git fetch upstream: %v", err)
	}

	if err := exec.Command("git", "branch", "-u", "upstream/master", "upstream").Run(); err != nil {
		return fmt.Errorf("git branch -u upstream/master upstream: %v", err)
	}

	return nil
}

func main() {
	if err := logic(); err != nil {
		log.Fatal(err)
	}
}
