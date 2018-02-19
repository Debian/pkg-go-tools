package main

import (
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

func applyPatches(dir string) error {
	patchDir, err := filepath.Abs(filepath.Join(dir, "debian", "patches"))
	if err != nil {
		return err
	}
	patches, err := ioutil.ReadFile(filepath.Join(patchDir, "series"))
	if err != nil {
		if os.IsNotExist(err) {
			return nil // no patches
		}
		return err
	}
	// validate series file (otherwise quilt exits with an error)
	hasPatches := false
	for _, patch := range strings.Split(strings.TrimSpace(string(patches)), "\n") {
		if strings.HasPrefix(patch, "#") {
			continue // skip comments
		}
		if patch == "" {
			continue // skip empty lines
		}
		hasPatches = true
		break
	}
	if !hasPatches {
		return nil // series file contains no valid patches
	}
	quilt := exec.Command("quilt", "push", "-a")
	quilt.Stderr = os.Stderr
	quilt.Dir = dir
	quilt.Env = []string{"QUILT_PATCHES=" + patchDir}
	if err := quilt.Run(); err != nil {
		return fmt.Errorf("%v in %v: %v", quilt.Args, quilt.Dir, err)
	}
	return nil
}
