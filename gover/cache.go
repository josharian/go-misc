// Copyright 2016 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
	"io/ioutil"
	"log"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"
)

var hashNameRe = regexp.MustCompile(`^[0-9a-f]{7,40}(\+[0-9a-f]{1,10})?$`)
var fullHashRe = regexp.MustCompile("^[0-9a-f]{40}$")
var hashPlusRe = regexp.MustCompile(`^[0-9a-f]{40}(\+[0-9a-f]{10})?$`)

// resolveName returns the path to the root of the named build and
// whether or not that path exists. It will log an error and exit if
// name is ambiguous. If the path does not exist, the returned path is
// where this build should be saved.
func resolveName(name string) (path string, ok bool) {
	// If the name exactly matches a saved version, return it.
	savePath := filepath.Join(*verDir, name)
	st, err := os.Stat(savePath)
	if err == nil && st.IsDir() {
		return savePath, true
	}

	// Otherwise, try to resolve it as an unambiguous hash prefix.
	if hashNameRe.MatchString(name) {
		nameParts := strings.SplitN(name, "+", 2)
		builds, err := listBuilds(0)
		if err != nil {
			log.Fatal(err)
		}

		var fullName string
		for _, b := range builds {
			if !strings.HasPrefix(b.commitHash, nameParts[0]) {
				continue
			}
			if (len(nameParts) == 1) != (b.deltaHash == "") {
				continue
			}
			if len(nameParts) > 1 && !strings.HasPrefix(b.deltaHash, nameParts[1]) {
				continue
			}

			// We found a match.
			if fullName != "" {
				log.Fatalf("ambiguous name `%s`", name)
			}
			fullName = b.fullName()
		}
		if fullName != "" {
			return filepath.Join(*verDir, fullName), true
		}
	}

	return savePath, false
}

type buildInfo struct {
	path       string
	commitHash string
	deltaHash  string
	names      []string
	commit     *commit
}

func (i buildInfo) fullName() string {
	if i.deltaHash == "" {
		return i.commitHash
	}
	return i.commitHash + "+" + i.deltaHash
}

func (i buildInfo) shortName() string {
	// TODO: Print more than 7 characters if necessary.
	if i.deltaHash == "" {
		return i.commitHash[:7]
	}
	return i.commitHash[:7] + "+" + i.deltaHash
}

type listFlags int

const (
	listNames listFlags = 1 << iota
	listCommit
)

func listBuilds(flags listFlags) ([]*buildInfo, error) {
	files, err := ioutil.ReadDir(*verDir)
	if os.IsNotExist(err) {
		return nil, nil
	} else if err != nil {
		return nil, err
	}

	// Collect the saved builds.
	builds := []*buildInfo{}
	var baseMap map[string]*buildInfo
	if flags&listNames != 0 {
		baseMap = make(map[string]*buildInfo)
	}
	for _, file := range files {
		if !file.IsDir() || !hashPlusRe.MatchString(file.Name()) {
			continue
		}
		nameParts := strings.SplitN(file.Name(), "+", 2)
		info := &buildInfo{path: filepath.Join(*verDir, file.Name()), commitHash: nameParts[0]}
		if len(nameParts) > 1 {
			info.deltaHash = nameParts[1]
		}

		builds = append(builds, info)
		if baseMap != nil {
			baseMap[file.Name()] = info
		}

		if flags&listCommit != 0 {
			commit, err := ioutil.ReadFile(filepath.Join(*verDir, file.Name(), "commit"))
			if err != nil {
				log.Fatal(err)
			} else {
				info.commit = parseCommit(commit)
			}
		}
	}

	// Collect the names for each build.
	if flags&listNames != 0 {
		for _, file := range files {
			if file.Mode()&os.ModeType == os.ModeSymlink {
				base, err := os.Readlink(filepath.Join(*verDir, file.Name()))
				if err != nil {
					continue
				}
				if info, ok := baseMap[base]; ok {
					info.names = append(info.names, file.Name())
				}
			}
		}
	}

	return builds, nil
}

type commit struct {
	authorDate time.Time
	topLine    string
}

func parseCommit(obj []byte) *commit {
	out := &commit{}
	lines := strings.Split(string(obj), "\n")
	for i, line := range lines {
		if strings.HasPrefix(line, "author ") {
			fs := strings.Fields(line)
			secs, err := strconv.ParseInt(fs[len(fs)-2], 10, 64)
			if err != nil {
				log.Fatalf("malformed author in commit: %s", err)
			}
			out.authorDate = time.Unix(secs, 0)
		}
		if len(line) == 0 {
			out.topLine = lines[i+1]
			break
		}
	}
	return out
}
