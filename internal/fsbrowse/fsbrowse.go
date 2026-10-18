// Package fsbrowse provides directory listings shared by local and SSH browsers.
package fsbrowse

import (
	"context"
	"os"
	"path/filepath"
	"sort"
)

type Entry struct {
	Name string
	Dir  bool
}

type Listing struct {
	Path    string
	Entries []Entry
}

type ListFunc func(context.Context, string) (Listing, error)

func Sort(entries []Entry) {
	sort.Slice(entries, func(i, j int) bool {
		if entries[i].Dir != entries[j].Dir {
			return entries[i].Dir
		}
		return entries[i].Name < entries[j].Name
	})
}

func Local(ctx context.Context, dir string) (Listing, error) {
	if dir == "" {
		dir, _ = os.Getwd()
	}
	abs, err := filepath.Abs(dir)
	if err != nil {
		return Listing{}, err
	}
	abs, err = filepath.EvalSymlinks(abs)
	if err != nil {
		return Listing{}, err
	}
	items, err := os.ReadDir(abs)
	if err != nil {
		return Listing{}, err
	}
	result := Listing{Path: filepath.ToSlash(abs)}
	for _, item := range items {
		if err := ctx.Err(); err != nil {
			return Listing{}, err
		}
		// Symlinks can be selected as files, but are never traversed.
		result.Entries = append(result.Entries, Entry{Name: item.Name(), Dir: item.IsDir()})
	}
	Sort(result.Entries)
	return result, nil
}
