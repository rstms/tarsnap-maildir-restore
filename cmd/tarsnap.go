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
	"bufio"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"github.com/spf13/viper"
)

const CMD_LENGTH_BUFFER = 2048
const CMD_LENGTH_MIN = 8192
const CMD_LENGTH_LIMIT = 32767

var LIST_FILENAME_PATTERN = regexp.MustCompile(`^\d{4}(?:-\d{2}){2}\.[^.]+\.([^.]+)\.file_list$`)
var FILE_LIST_PATTERN = regexp.MustCompile(`^(?:\S+\s+){4}(\d+)\s+[^.]+(\..+)$`)
var USER_PATTERN = regexp.MustCompile(`^\./([^/]+)/Maildir/.*`)
var MAILDIR_PATTERN = regexp.MustCompile(`^\./[^/]+/Maildir/([^/]+).*$`)

//var CUR_PATTERN = regexp.MustCompile(`^\./[^/]+/Maildir(/[^/]+){0,1}/cur$`)
//var CUR_NEW_TMP_PATTERN = regexp.MustCompile(`^\./[^/]+/Maildir(/[^/]+){0,1}/(cur|new|tmp)$`)
//var DIR_PATTERN = regexp.MustCompile(`^.*/$`)
//var INBOX_PATTERN = regexp.MustCompile(`^\./[^/]+/Maildir/[^.][^/]+/.*$`)
//var ROOTFILE_PATTERN = regexp.MustCompile(`^(./[^/]*/Maildir/[^.][^/]*)$`)
//var ROOTDIR_PATTERN = regexp.MustCompile(`^(./[^/]*/Maildir/[^.][^/]*/).*$`)
//var NEW_TMP_PATTERN = regexp.MustCompile(`^.*/(new|tmp)/$`)
//var DIR_MAP_PATTERN = regexp.MustCompile(`^(\d+)\s+(\S+).*$`)
//var MAILDIR_LIST_PATTERN = regexp.MustCompile(`^\./([^/]+)/Maildir/\.([^/]+)/cur$`)
//var CUR_MESSAGE_PATTERN = regexp.MustCompile(`^\./[^/]+/Maildir(/[^/]+){0,1}/cur/[^/]+$`)
//var NEW_MESSAGE_PATTERN = regexp.MustCompile(`^\./[^/]+/Maildir(/[^/]+){0,1}/new/.+$`)
//var TMP_MESSAGE_PATTERN = regexp.MustCompile(`^\./[^/]+/Maildir(/[^/]+){0,1}/tmp/.+$`)

type MaildirFile struct {
	Name string
	Size int64
}

type Maildir struct {
	Files []MaildirFile
}

func (m *Maildir) AddFile(file string, size int64) {
	m.Files = append(m.Files, MaildirFile{Name: file, Size: size})
}

type User struct {
	Maildirs map[string]*Maildir
}

func (u *User) getMaildir(name string) *Maildir {
	_, ok := u.Maildirs[name]
	if !ok {
		u.Maildirs[name] = &Maildir{
			Files: []MaildirFile{},
		}
	}
	return u.Maildirs[name]
}

type Tarsnap struct {
	Archive       string
	Users         map[string]*User
	lengthLimit   int
	userFilter    *regexp.Regexp
	maildirFilter *regexp.Regexp
	destDir       string
	skipLogged    map[string]bool
	debug         bool
	verbose       bool
	json          bool
	dryrun        bool
}

func NewTarsnap(name string) (*Tarsnap, error) {
	viper.SetDefault("tarsnap_command", "tarsnap")
	viper.SetDefault("user", ".*")
	userFilter, err := regexp.Compile(viper.GetString("user"))
	if err != nil {
		return nil, fmt.Errorf("failed user filter regexp compile: %v", err)
	}
	viper.SetDefault("maildir", ".*")
	maildirFilter, err := regexp.Compile(viper.GetString("maildir"))
	if err != nil {
		return nil, fmt.Errorf("failed maildir filter regexp compile: %v", err)
	}

	t := Tarsnap{
		Archive:       name,
		Users:         make(map[string]*User),
		lengthLimit:   CMD_LENGTH_LIMIT,
		userFilter:    userFilter,
		maildirFilter: maildirFilter,
		destDir:       ExpandPath(viper.GetString("output_dir")),
		skipLogged:    make(map[string]bool),
		debug:         viper.GetBool("debug"),
		verbose:       viper.GetBool("verbose"),
		json:          viper.GetBool("json"),
		dryrun:        viper.GetBool("dryrun"),
	}

	/*
		err = t.setLengthLimit()
		if err != nil {
			return nil, err
		}
	*/

	err = t.initialize()
	if err != nil {
		return nil, err
	}

	return &t, nil
}

func (t *Tarsnap) setLengthLimit() error {

	envSize := 0
	environ := os.Environ()
	for _, name := range environ {
		value := os.Getenv(name)
		envSize += len(name) + len(value) + 2
	}

	proc := NewProcess("getconf", []string{"ARG_MAX"})
	stdout, _, err := proc.Run()
	if err != nil {
		return fmt.Errorf("failed reading ARG_MAX: %v", err)
	}
	argMax, err := strconv.Atoi(strings.TrimSpace(stdout))
	if err != nil {
		return fmt.Errorf("failed parsing getconf output: %v", err)
	}

	t.lengthLimit = argMax - (envSize + CMD_LENGTH_BUFFER)

	if t.lengthLimit < CMD_LENGTH_MIN {
		return fmt.Errorf("command length below limit: %d", t.lengthLimit)
	}

	panic(fmt.Sprintf("lengthLimit: %d", t.lengthLimit))

	return nil
}

func (t *Tarsnap) getUser(name string) *User {
	_, ok := t.Users[name]
	if !ok {
		t.Users[name] = &User{Maildirs: make(map[string]*Maildir)}
	}
	return t.Users[name]
}

func (t *Tarsnap) Restore() error {

	restores := NewProcessSet()
	files := []string{}
	var cmdLength int
	var size int64
	for userName, user := range t.Users {
		for maildirName, maildir := range user.Maildirs {
			archiveName := fmt.Sprintf("%s.%s.maildir", t.Archive, userName)
			for _, file := range maildir.Files {
				if cmdLength+len(file.Name) > t.lengthLimit {
					err := restores.AddRestore(archiveName, userName, maildirName, files, size)
					if err != nil {
						return err
					}
					size = 0
					cmdLength = 0
					files = []string{}
				} else {
					files = append(files, file.Name)
					cmdLength += len(file.Name)
					size += file.Size
				}
			}
			if len(files) > 0 {
				err := restores.AddRestore(archiveName, userName, maildirName, files, size)
				if err != nil {
					return err
				}
			}
		}
	}

	if t.dryrun {
		return nil
	}

	err := restores.Run()
	if err != nil {
		return err
	}

	return nil
}

func (t *Tarsnap) parseFile(userName, sizeStr, filename string) error {

	size, err := strconv.ParseInt(sizeStr, 10, 64)
	if err != nil {
		return fmt.Errorf("failed converting file size: %v", err)
	}

	match := USER_PATTERN.FindStringSubmatch(filename)
	if len(match) != 2 {
		return fmt.Errorf("failed parsing username from: %s", filename)
	}
	if userName != match[1] {
		return fmt.Errorf("unexpected username '%s' in %s", match[1], filename)
	}

	user := t.getUser(userName)

	maildirName := ""
	match = MAILDIR_PATTERN.FindStringSubmatch(filename)

	if t.debug {
		log.Printf("\nLINE: %s\n", filename)
		log.Printf("MAILDIR_PATTERN: %d %v\n", len(match), match)
	}

	if len(match) == 2 {
		maildirName = match[1]
	}
	if maildirName == "" || !strings.HasPrefix(maildirName, ".") {
		maildirName = "INBOX"
	}

	if t.debug {
		log.Printf("MAILDIR: %s\n", maildirName)
	}

	if !t.maildirFilter.MatchString(maildirName) {
		if t.verbose {
			_, ok := t.skipLogged[maildirName]
			if !ok {
				log.Printf("skipping filtered maildirName: %s\n", maildirName)
				t.skipLogged[maildirName] = true
			}
		}
		return nil
	}

	maildir := user.getMaildir(maildirName)

	if t.debug {
		log.Printf("add %s %s %s\n", userName, maildirName, filename)
	}

	maildir.AddFile(filename, size)

	return nil
}

func (t *Tarsnap) initialize() error {

	metadataDir := ExpandPath(viper.GetString("metadata_dir"))

	if t.verbose && metadataDir != "" {
		log.Printf("using preloaded metadata in: %s", metadataDir)
	}

	if metadataDir == "" {
		dir, err := os.MkdirTemp("", "tarsnap.metadata.*")
		if err != nil {
			return err
		}
		metadataDir = dir
		//defer os.RemoveAll(metadataDir)
		metadataArchive := fmt.Sprintf("%s.metadata", t.Archive)
		args := []string{
			"-x",
			"--keyfile", ExpandPath(viper.GetString("keyfile")),
			"-f", metadataArchive,
			"-C", metadataDir,
		}
		if t.verbose {
			log.Printf("extracting metadata: %s\n", metadataArchive)
		}
		p := NewTarsnapProcess(args)
		_, _, err = p.Run()
		if err != nil {
			return fmt.Errorf("metadata extract failed: %v", err)
		}
	}
	if t.verbose {
		log.Printf("reading metadata dir: %s\n", metadataDir)
	}
	entries, err := os.ReadDir(metadataDir)
	if err != nil {
		return fmt.Errorf("failed reading metadata files: %v", err)
	}
	for _, entry := range entries {
		if entry.Type().IsRegular() {
			if t.verbose {
				log.Printf("reading metadata file: %s\n", entry.Name())
			}
			err := t.readFileList(filepath.Join(metadataDir, entry.Name()))
			if err != nil {
				return err
			}
		}
	}
	return nil
}

func (t *Tarsnap) readFileList(pathname string) error {

	_, filename := filepath.Split(pathname)

	if t.verbose {
		log.Printf("reading %s\n", filename)
	}

	match := LIST_FILENAME_PATTERN.FindStringSubmatch(filename)
	if len(match) != 2 {
		return fmt.Errorf("file_list filename parse failed: %d %v", len(match), match)
	}
	userName := match[1]

	if !t.userFilter.MatchString(userName) {
		if t.verbose {
			log.Printf("skipping filtered username: %s\n", userName)
		}
		return nil
	}

	file, err := os.Open(pathname)
	if err != nil {
		return err
	}
	defer file.Close()
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := scanner.Text()
		match := FILE_LIST_PATTERN.FindStringSubmatch(line)
		if len(match) != 3 {
			return fmt.Errorf("file_list line parse failed: %d %v", len(match), match)
		}
		err := t.parseFile(userName, match[1], match[2])
		if err != nil {
			return err
		}
	}
	return nil
}

func (t *Tarsnap) Files() []string {
	files := []string{}
	for _, user := range t.Users {
		for _, maildir := range user.Maildirs {
			for _, file := range maildir.Files {
				if !strings.HasSuffix(file.Name, "/") {
					files = append(files, file.Name)
				}
			}
		}
	}
	return files
}
