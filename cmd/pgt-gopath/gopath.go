// Program pgt-gopath constructs a Go workspace src directory from the Debian
// unstable archive. See https://golang.org/doc/code.html#Workspaces
//
// This is useful, for example, for quick continuous integration: gone is the
// computationally intensive step of identifying reverse dependencies of Debian
// packages, and gone is the overhead of installing .deb packages (orders of
// magnitude slower than pgt-gopath). One can directly use the go tool, which
// builds/tests quickly and with caching. See
// https://pkg-go.alioth.debian.org/ci.html for more details on how Debian uses
// this.
//
// Even users outside of Debian can use such a Go workspace to obtain a
// reasonably large body of software with real-world usage, perhaps to run
// regression tests when doing changes to the Go standard library.
//
// pgt-gopath leverages apt-cacher-ng(8) for caching: each run of pgt-gopath
// constructs an entirely new GOPATH/src directory, consuming many .orig
// tarballs from the Debian archive. This typically takes less than 10 seconds
// on a modern computer.
//
// The resulting src directory is suffixed with the UNIX timestamp of the
// release metadata’s last modified timestamp. In case the last modified
// timestamp matches the current on-disk timestamp, pgt-gopath immediately exits
// successfully. Hence, it can be run in a minutely cronjob.
//
// This last modified timestamp is printed to stdout (whereas log messages are
// printed to stderr). This allows for post-processing (e.g. chmod) and
// atomically updating the Go workspace via:
//
//    #!/bin/bash
//    # Not safe for concurrent execution: wrap in flock(1) or a systemd service.
//    set -e
//
//    # Update pgt-gopath to pick up fixes:
//    go get -u github.com/Debian/pkg-go-team/cmd/pgt-gopath
//
//    mkdir -p /srv/gopath
//    cd /srv/gopath
//
//    # Create a new src-<timestamp> directory:
//    latest=$(pgt-gopath)
//
//    # Atomically update the src symlink:
//    ln -snf src-${latest} new_src
//    mv -T new_src src
//
//    # Clean up all old src-<timestamp> directories:
//    rm -rf -- $(ls -d src-* | grep -v "^src-$latest\$")
package main

import (
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"strings"
	"time"

	"golang.org/x/sync/errgroup"
	"pault.ag/go/archive"
	"pault.ag/go/debian/control"
)

var (
	debianMirror = flag.String("debian_mirror",
		"http://localhost:3142/deb.debian.org/debian",
		"HTTP URL of the Debian mirror to use. Install apt-cacher-ng(8) instead of specifying this flag to massively cut down on bandwidth (useful even with fast links).")
)

var ignored = map[string]bool{
	"kxd":         true, // not go-gettable, but also no dependencies other than the stdlib. ignore for now.
	"golang-1.6":  true, // compiler
	"golang-1.7":  true, // compiler
	"golang-1.8":  true, // compiler
	"golang-1.9":  true, // compiler
	"golang-1.10": true, // compiler
}

// rewrite maps from Debian source package to Go-Import-Path. Each entry is
// annotated with a URL of the upstream-submitted patch and should be removed
// once that patch is merged.
var rewrite = map[string]string{
	"gitlab-workhorse":                  "gitlab.com/gitlab-org/gitlab-workhorse", // https://bugs.debian.org/890056
	"pluginhook":                        "github.com/progrium/pluginhook",         // https://bugs.debian.org/890057
	"golang-github-gosexy-gettext":      "github.com/gosexy/gettext",              // https://bugs.debian.org/890058
	"mongo-tools":                       "github.com/mongodb/mongo-tools",         // https://bugs.debian.org/890059
	"golang-github-mvo5-goconfigparser": "github.com/mvo5/goconfigparser",         // https://github.com/vorlonofportland/goconfigparser/pull/1
}

func logic() error {
	const parallel = 20

	start := time.Now()
	tempdir, err := ioutil.TempDir(".", "src-tmp-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(tempdir)
	g := &archive.Downloader{
		Parallel:            parallel,
		MaxTransientRetries: 3,
		Mirror:              strings.TrimSuffix(*debianMirror, "/"),
		// TODO: insecure flag to skip GPG verification on systems missing DebianArchiveKeyring
	}
	release, rd, err := g.Release("unstable")
	if err != nil {
		return err
	}
	timestamp := fmt.Sprintf("%d", rd.LastModified.Unix())
	if _, err := os.Stat("src-" + timestamp); err == nil {
		fmt.Println(timestamp)
		return nil
	}
	const sourcesPath = "main/source/Sources.gz"
	fhs := release.Indices()[sourcesPath]
	if len(fhs) == 0 {
		return fmt.Errorf("%s not found", sourcesPath)
	}
	srcs, err := downloadSources(rd, fhs[0])
	if err != nil {
		return err
	}
	log.Printf("loaded %d source packages in %v", len(srcs), time.Since(start))

	// Process the repos, 20 at a time.
	var eg errgroup.Group
	semaphore := make(chan struct{}, parallel)

	for _, src := range srcs {
		if ignored[src.Package] {
			continue
		}
		if src.ExtraSourceOnly {
			continue // see https://bugs.debian.org/814156 for details
		}
		importPath := src.GoImportPath
		if to, ok := rewrite[src.Package]; ok {
			importPath = to
		}
		if importPath == "" {
			// TODO: document what this means
			log.Printf("package src:%s is missing xs-go-import-path", src.Package)
			continue
		}
		if importPath == "github.com/spf13/cobra" {
			log.Printf("pkg = %v", src.Package)
		}
		if src.Package == "golang-github-dnephin-cobra" {
			continue // TODO: file bug: same import path: golang-github-dnephin-cobra vs. golang-github-spf13-cobra
		}

		if src.Package == "docker-containerd" {
			continue // TODO: file bug: duplicate package: src:docker-containerd vs. src:containerd
		}

		src := src // copy
		eg.Go(func() error {
			semaphore <- struct{}{}
			defer func() { <-semaphore }()

			var origTar, debTar control.FileHash
			for _, c := range src.Checksums() {
				if strings.HasSuffix(c.Filename, ".asc") {
					continue // skip signature files
				}
				if strings.Contains(c.Filename, ".orig.tar.") {
					origTar = c
				} else if strings.Contains(c.Filename, ".debian.tar.") {
					debTar = c
				}
			}
			if origTar.Filename == "" {
				log.Printf("ERROR: src:%s is missing .orig.tar. file", src.Package)
				return nil
			}
			if debTar.Filename == "" {
				log.Printf("ERROR: src:%s is missing .debian.tar. file", src.Package)
				return nil
			}
			origTar.Filename = path.Join(src.Directory, origTar.Filename)
			origTarTmp, err := g.TempFile(origTar)
			if err != nil {
				return fmt.Errorf("src:%s: download(origTar=%s): %v", src.Package, origTar.Filename, err)
			}
			if err := origTarTmp.Close(); err != nil {
				return err
			}
			defer os.Remove(origTarTmp.Name())

			debTar.Filename = path.Join(src.Directory, debTar.Filename)
			debTarTmp, err := g.TempFile(debTar)
			if err != nil {
				return fmt.Errorf("src:%s: download(debTar=%s): %v", src.Package, debTar.Filename, err)
			}
			if err := debTarTmp.Close(); err != nil {
				return err
			}
			defer os.Remove(debTarTmp.Name())

			destRepo := filepath.Join(tempdir, strings.Split(importPath, ",")[0])
			if err := unpack(destRepo, origTarTmp.Name()); err != nil {
				return fmt.Errorf("unpacking orig tarball: %v", err)
			}
			if err := unpack(filepath.Join(destRepo, "debian"), debTarTmp.Name()); err != nil {
				return fmt.Errorf("unpacking debian tarball: %v", err)
			}
			if err := applyPatches(destRepo); err != nil {
				return fmt.Errorf("applying patches: %v", err)
			}
			if err := createLinks(tempdir, destRepo); err != nil {
				return fmt.Errorf("creating links: %v", err)
			}
			if err := cleanFiles(destRepo); err != nil {
				return fmt.Errorf("cleaning files: %v", err)
			}

			// enforce resulting files are world-readable for building with unprivileged
			// users (e.g. golang-github-svent-go-nbreader comes with a debian tarball
			// without word-readable bits). Permissions were copied from “apt source”.
			chmod := exec.Command("chmod", "-R", "--", "u+r+w+X,g+r-w+X,o+r-w+X", destRepo)
			chmod.Stderr = os.Stderr
			if err := chmod.Run(); err != nil {
				return fmt.Errorf("%v: %v", chmod.Args, err)
			}

			// Persist all hashes which define the package into the
			// debian/.hashes file. This can be used by downstream software to
			// detect package changes for caching.
			hashes := []byte(strings.Join([]string{
				origTar.Filename + "=" + origTar.Hash,
				debTar.Filename + "=" + debTar.Hash,
			}, "\n") + "\n")
			if err := ioutil.WriteFile(filepath.Join(destRepo, "debian", ".hashes"), hashes, 0644); err != nil {
				return err
			}

			return nil
		})
	}

	if err := eg.Wait(); err != nil {
		return err
	}

	if err := os.Rename(tempdir, "src-"+timestamp); err != nil {
		return err
	}

	fmt.Println(timestamp)

	return nil
}

func main() {
	flag.Parse()
	if err := logic(); err != nil {
		log.Fatal(err)
	}
}
