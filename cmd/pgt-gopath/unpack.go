package main

import (
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"
)

func unpack(dest, fn string) error {
	if err := os.MkdirAll(dest, 0755); err != nil {
		return err
	}
	// Some packages (e.g. golang-github-nwidger-jsoncolor) have .orig.tar files
	// which don’t place their files in a subdirectory, so we need to check for
	// that:
	tf := exec.Command("tar", "tf", fn)
	stripComponents := 1
	out, err := tf.Output()
	if err != nil {
		return err
	}
	for _, line := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		if !strings.Contains(line, "/") {
			stripComponents = 0
			break
		}
	}

	tar := exec.Command("tar",
		"xf",
		fn,
		"-C", dest,
		"--strip-components="+strconv.Itoa(stripComponents),
	)
	tar.Stderr = os.Stderr
	if err := tar.Run(); err != nil {
		return fmt.Errorf("%v: %v", tar.Args, err)
	}

	return nil
}
