/*
Copyright © 2025 Matt Krueger <mkrueger@rstms.net>
All rights reserved.

Redistribution and use in source and binary forms, with or without
modification, are permitted provided that the following conditions are met:

 1. Redistributions of source code must retain the above copyright notice,
    this list of conditions and the following disclaimer.

 2. Redistributions in binary form must reproduce the above copyright notice,
    this list of conditions and the following disclaimer in the documentation
    and/or other materials provided with the distribution.

 3. Neither the name of the copyright holder nor the names of its contributors
    may be used to endorse or promote products derived from this software
    without specific prior written permission.

THIS SOFTWARE IS PROVIDED BY THE COPYRIGHT HOLDERS AND CONTRIBUTORS "AS IS"
AND ANY EXPRESS OR IMPLIED WARRANTIES, INCLUDING, BUT NOT LIMITED TO, THE
IMPLIED WARRANTIES OF MERCHANTABILITY AND FITNESS FOR A PARTICULAR PURPOSE
ARE DISCLAIMED. IN NO EVENT SHALL THE COPYRIGHT HOLDER OR CONTRIBUTORS BE
LIABLE FOR ANY DIRECT, INDIRECT, INCIDENTAL, SPECIAL, EXEMPLARY, OR
CONSEQUENTIAL DAMAGES (INCLUDING, BUT NOT LIMITED TO, PROCUREMENT OF
SUBSTITUTE GOODS OR SERVICES; LOSS OF USE, DATA, OR PROFITS; OR BUSINESS
INTERRUPTION) HOWEVER CAUSED AND ON ANY THEORY OF LIABILITY, WHETHER IN
CONTRACT, STRICT LIABILITY, OR TORT (INCLUDING NEGLIGENCE OR OTHERWISE)
ARISING IN ANY WAY OUT OF THE USE OF THIS SOFTWARE, EVEN IF ADVISED OF THE
POSSIBILITY OF SUCH DAMAGE.
*/

package cmd

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"github.com/spf13/viper"
)

var USER_PATTERN = regexp.MustCompile(`^\./([^/]+)/Maildir/.*`)
var CUR_PATTERN = regexp.MustCompile(`^\./[^/]+/Maildir(/[^/]+){0,1}/cur$`)

var CUR_NEW_TMP_PATTERN = regexp.MustCompile(`^\./[^/]+/Maildir(/[^/]+){0,1}/(cur|new|tmp)$`)

var DIR_PATTERN = regexp.MustCompile(`^.*/$`)

var INBOX_PATTERN = regexp.MustCompile(`^\./[^/]+/Maildir/[^.][^/]+/.*$`)

var MAILDIR_PATTERN = regexp.MustCompile(`^\./[^/]+/Maildir/([^/]+).*$`)
var ROOTFILE_PATTERN = regexp.MustCompile(`^(./[^/]*/Maildir/[^.][^/]*)$`)
var ROOTDIR_PATTERN = regexp.MustCompile(`^(./[^/]*/Maildir/[^.][^/]*/).*$`)
var NEW_TMP_PATTERN = regexp.MustCompile(`^.*/(new|tmp)/$`)
var DIR_MAP_PATTERN = regexp.MustCompile(`^(\d+)\s+(\S+).*$`)
var MAILDIR_LIST_PATTERN = regexp.MustCompile(`^\./([^/]+)/Maildir/\.([^/]+)/cur$`)

var CUR_MESSAGE_PATTERN = regexp.MustCompile(`^\./[^/]+/Maildir(/[^/]+){0,1}/cur/[^/]+$`)
var NEW_MESSAGE_PATTERN = regexp.MustCompile(`^\./[^/]+/Maildir(/[^/]+){0,1}/new/.+$`)
var TMP_MESSAGE_PATTERN = regexp.MustCompile(`^\./[^/]+/Maildir(/[^/]+){0,1}/tmp/.+$`)

type Maildir struct {
	Name     string
	Patterns []string
	Files    []string
	Blocks   int
	ScanPath string
}

func NewMaildir(user, name string, blocks int) *Maildir {

	m := Maildir{
		Name:     name,
		Patterns: []string{},
		Files:    []string{},
		Blocks:   blocks,
	}
	if name == "INBOX" {
		for _, dir := range []string{"cur", "tmp", "new"} {
			m.Patterns = append(m.Patterns, "./"+filepath.Join(user, "Maildir", dir, "*"))
		}
		m.ScanPath = filepath.Join(user, "Maildir", "cur")
	} else {
		m.Patterns = append(m.Patterns, "./"+filepath.Join(user, "Maildir", name, "*"))
		m.ScanPath = filepath.Join(user, "Maildir", name, "cur")
	}
	return &m
}

func (m *Maildir) AddFile(file string) {
	m.Files = append(m.Files, file)
}

func (m *Maildir) AddPattern(file string) {
	m.Patterns = append(m.Patterns, file)
}

type User struct {
	Name     string
	Maildirs map[string]*Maildir
}

func NewUser(name string) *User {
	u := User{
		Name:     name,
		Maildirs: make(map[string]*Maildir),
	}
	return &u
}

type Tarsnap struct {
	Archive       string
	Files         []string
	Dirs          map[string]int
	Users         map[string]*User
	debug         bool
	verbose       bool
	json          bool
	dryrun        bool
	userFilter    *regexp.Regexp
	maildirFilter *regexp.Regexp
	destDir       string
}

func NewTarsnap(archiveName string) (*Tarsnap, error) {
	viper.SetDefault("user", ".*")
	userFilter, err := regexp.Compile(viper.GetString("user"))
	if err != nil {
		return nil, err
	}
	viper.SetDefault("maildir", ".*")
	maildirFilter, err := regexp.Compile(viper.GetString("maildir"))
	if err != nil {
		return nil, err
	}

	t := Tarsnap{
		Archive:       archiveName,
		Files:         []string{},
		Dirs:          make(map[string]int),
		Users:         make(map[string]*User),
		debug:         viper.GetBool("debug"),
		verbose:       viper.GetBool("verbose"),
		json:          viper.GetBool("json"),
		dryrun:        viper.GetBool("dryrun"),
		userFilter:    userFilter,
		maildirFilter: maildirFilter,
		destDir:       viper.GetString("dest_dir"),
	}
	err = t.initialize()
	if err != nil {
		return nil, err
	}
	return &t, nil
}

func (t *Tarsnap) mapFiles() error {

	for _, line := range t.Files {
		err := t.parseFileLine(line)
		if err != nil {
			return err
		}
	}
	return nil
}

func (t *Tarsnap) Restore() error {

	restores := NewProcessSet()

	for username, user := range t.Users {
		if t.verbose {
			log.Printf("%s\n", username)
		}
		for _, maildir := range user.Maildirs {
			if t.verbose {
				for _, pattern := range maildir.Patterns {
					log.Printf("\t%s\n", pattern)
				}
			}
			err := restores.AddRestore(t.Archive, maildir.Patterns)
			if err != nil {
				return err
			}
		}
	}

	if !t.dryrun {
		err := restores.Wait()
		if err != nil {
			return err
		}
	}
	return nil
}

func (t *Tarsnap) parseFileLine(line string) error {

	if line == "" || line == "." || line == "./file_list" || line == "./dir_list" {
		return nil
	}

	if CUR_NEW_TMP_PATTERN.MatchString(line) {
		// TODO: be sure these dirs are present in a pattern of one of the maildirs
		return nil
	}

	username := ""
	match := USER_PATTERN.FindStringSubmatch(line)
	if len(match) == 2 {
		username = match[1]
	} else {
		return nil
	}

	if !t.userFilter.MatchString(username) {
		return nil
	}

	maildir := ""
	match = MAILDIR_PATTERN.FindStringSubmatch(line)
	if len(match) == 2 {
		maildir = match[1]
	}
	if maildir == "" {
		log.Fatalf("no maildir: %s\n", line)
	}

	if !strings.HasPrefix(maildir, ".") {
		maildir = "INBOX"
	}

	if !t.maildirFilter.MatchString(maildir) {
		return nil
	}

	if CUR_MESSAGE_PATTERN.MatchString(line) {
		if t.debug {
			t.Users[username].Maildirs[maildir].AddFile(line)
		}
		return nil
	}
	if TMP_MESSAGE_PATTERN.MatchString(line) {
		if t.debug {
			log.Printf("TMP: %s %s %s\n", username, maildir, line)
		}
		return nil
	}
	if NEW_MESSAGE_PATTERN.MatchString(line) {
		if t.debug {
			log.Printf("NEW: %s %s %s\n", username, maildir, line)
		}
		return nil
	}

	if maildir == "INBOX" {
		t.Users[username].Maildirs[maildir].AddPattern(line)
		return nil
	}

	if t.debug {
		fmt.Printf("OTHER: %s %s %s\n", username, maildir, line)
	}

	return nil
}

func (t *Tarsnap) initialize() error {

	fileListFile := viper.GetString("file_list")
	if t.verbose && fileListFile != "" {
		log.Printf("using local file_list: %s", fileListFile)
	}

	dirListFile := viper.GetString("dir_list")
	if t.verbose && dirListFile != "" {
		log.Printf("using local dir_list: %s", dirListFile)
	}

	if fileListFile == "" || dirListFile == "" {
		destDir, err := os.MkdirTemp("", "restore.*")
		if err != nil {
			return err
		}
		defer os.RemoveAll(destDir)
		args := []string{
			"-x",
			"--keyfile", viper.GetString("keyfile"),
			"-f", t.Archive,
			"--fast-read",
			"-C", destDir,
		}
		if fileListFile == "" {
			args = append(args, "./file_list")
			fileListFile = filepath.Join(destDir, "file_list")
		}
		if dirListFile == "" {
			args = append(args, "./dir_list")
			dirListFile = filepath.Join(destDir, "dir_list")
		}
		log.Println("extracting metadadata")

		p := NewTarsnapProcess(args)
		_, _, err = p.Run()
		if err != nil {
			return fmt.Errorf("metadata extract failed: %v", err)
		}
	}

	err := t.readFiles(fileListFile)
	if err != nil {
		return err
	}

	err = t.readDirs(dirListFile)
	if err != nil {
		return err
	}

	err = t.mapMaildirs()
	if err != nil {
		return err
	}

	err = t.mapFiles()
	if err != nil {
		return err
	}

	return nil
}

func (t *Tarsnap) readFiles(filename string) error {
	t.Files = []string{}
	data, err := os.ReadFile(ExpandPath(filename))
	if err != nil {
		return err
	}
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if line != "" {
			t.Files = append(t.Files, line)
		}
	}
	return nil
}

func (t *Tarsnap) readDirs(filename string) error {
	data, err := os.ReadFile(ExpandPath(filename))
	if err != nil {
		return err
	}
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		match := DIR_MAP_PATTERN.FindStringSubmatch(line)
		if len(match) == 3 {
			size, err := strconv.Atoi(match[1])
			if err != nil {
				return err
			}
			t.Dirs[match[2]] = size
		}
	}
	return nil
}

func (t *Tarsnap) mapMaildirs() error {
	for _, line := range t.Files {
		if CUR_PATTERN.MatchString(line) {
			match := USER_PATTERN.FindStringSubmatch(line)
			if len(match) != 2 {
				log.Fatalf("no user in '%s'", line)
			}
			user := match[1]

			if !t.userFilter.MatchString(user) {
				continue
			}

			_, ok := t.Users[user]
			if !ok {
				t.Users[user] = NewUser(user)
			}

			var maildir string
			match = MAILDIR_PATTERN.FindStringSubmatch(line)
			if len(match) == 2 {
				maildir = match[1]
			}
			if maildir == "cur" {
				maildir = "INBOX"
			}

			if !t.maildirFilter.MatchString(maildir) {
				continue
			}

			blocks := t.Dirs[line]

			_, ok = t.Users[user].Maildirs[maildir]
			if !ok {
				t.Users[user].Maildirs[maildir] = NewMaildir(user, maildir, blocks)
			}
		}
	}
	return nil
}

func NewTarsnapProcess(args []string) *Process {
	cmd := viper.GetString("tarsnap_command")
	if cmd == "" {
		cmd = "tarsnap"
	}
	cmdline := append(strings.Split(cmd, " "), args...)
	return NewProcess(cmdline[0], cmdline[1:])
}
