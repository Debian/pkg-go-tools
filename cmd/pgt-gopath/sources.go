package main

import (
	"fmt"
	"io"
	"os"
	"sort"
	"strings"

	"pault.ag/go/archive"
	"pault.ag/go/debian/control"
	"pault.ag/go/debian/dependency"
	"pault.ag/go/debian/version"
)

// sourceIndex contains precisely the fields we are interested in, resulting in
// a more memory- and CPU-efficient parsing than using control.SourceIndex.
type sourceIndex struct {
	control.BestChecksums

	BuildDepends    dependency.Dependency `control:"Build-Depends"`
	GoImportPath    string                `control:"Go-Import-Path"`
	ExtraSourceOnly bool                  `control:"Extra-Source-Only"`
	Package         string
	Version         version.Version
	Directory       string
}

func (src *sourceIndex) importPath() string {
	importPath := src.GoImportPath
	if to, ok := rewrite[src.Package]; ok {
		importPath = to
	}
	return strings.Split(importPath, ",")[0]
}

func dependsOnGo(sidx *sourceIndex) bool {
	if sidx.GoImportPath != "" {
		return true
	}
	for _, possi := range sidx.BuildDepends.GetAllPossibilities() {
		if possi.Name == "golang-go" || possi.Name == "golang-any" ||
			possi.Name == "golang" /* incorrect, but e.g. golang-gocapability-dev uses it */ {
			return true
		}
	}
	return false
}

func loadSources(r io.Reader) ([]sourceIndex, error) {
	var temp []sourceIndex
	if err := control.Unmarshal(&temp, r); err != nil {
		return nil, fmt.Errorf("unmarshal: %v", err)
	}
	byName := make(map[string][]sourceIndex)
	for _, src := range temp {
		if !dependsOnGo(&src) {
			continue
		}
		byName[src.Package] = append(byName[src.Package], src)
	}
	filtered := make([]sourceIndex, 0, len(byName))
	for _, srcs := range byName {
		if len(srcs) > 1 {
			// Sort such that srcs[0] is the most recent version:
			sort.Slice(srcs, func(i, j int) bool { return version.Compare(srcs[i].Version, srcs[j].Version) > 0 })
		}
		filtered = append(filtered, srcs[0])
	}
	return filtered, nil
}

func downloadSources(r *archive.ReleaseDownloader, sourcesHash control.FileHash) ([]sourceIndex, error) {
	f, err := r.TempFile(sourcesHash)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	defer os.Remove(f.Name())

	srcs, err := loadSources(f)
	if err != nil {
		return nil, err
	}
	return srcs, nil
}
