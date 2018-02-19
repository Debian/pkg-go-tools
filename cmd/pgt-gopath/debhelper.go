package main

import (
	"io/ioutil"
	"log"
	"os"
	"path/filepath"
	"strings"
)

func dhFilesForPackage(dir, typ string) ([]string, error) {
	fis, err := ioutil.ReadDir(filepath.Join(dir, "debian"))
	if err != nil {
		return nil, err
	}
	var names []string
	for _, fi := range fis {
		if fi.Name() != typ && !strings.HasSuffix(fi.Name(), typ) {
			continue
		}
		names = append(names, fi.Name())
	}
	return names, nil
}

func createLinks(destdir, dir string) error {
	names, err := dhFilesForPackage(dir, "links")
	if err != nil {
		return err
	}
	for _, name := range names {
		links, err := ioutil.ReadFile(filepath.Join(dir, "debian", name))
		if err != nil {
			return err
		}
		for _, line := range strings.Split(strings.TrimSpace(string(links)), "\n") {
			if !strings.HasPrefix(line, "usr/share/gocode/src") {
				continue
			}
			parts := strings.Split(line, " ")
			if got, want := len(parts), 2; got != want {
				log.Printf("Skipping link line %q: unexpected number of parts: got %d, want %d", line, got, want)
				continue
			}
			oldname := filepath.Join(destdir, strings.TrimPrefix(parts[0], "usr/share/gocode/src"))
			newname := filepath.Join(destdir, strings.TrimPrefix(parts[1], "usr/share/gocode/src"))
			if err := os.MkdirAll(filepath.Dir(newname), 0755); err != nil {
				return err
			}
			rel, err := filepath.Rel(filepath.Dir(newname), oldname)
			if err != nil {
				return err
			}
			if err := os.Symlink(rel, newname); err != nil && !os.IsExist(err) {
				return err
			}
		}
	}
	return nil
}

func cleanFiles(dir string) error {
	names, err := dhFilesForPackage(dir, "clean")
	if err != nil {
		return err
	}
	for _, name := range names {
		clean, err := ioutil.ReadFile(filepath.Join(dir, "debian", name))
		if err != nil {
			return err
		}
		for _, line := range strings.Split(strings.TrimSpace(string(clean)), "\n") {
			if line == "" {
				continue
			}
			if err := os.Remove(filepath.Join(dir, line)); err != nil {
				if os.IsNotExist(err) {
					continue
				}
				return err
			}
		}
	}
	return nil
}
