package cmd

import (
	"bytes"
	"fmt"
	"github.com/schollz/progressbar/v3"
	"github.com/spf13/viper"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

const PROCESS_COUNT = 8

type Process struct {
	CommandLine string
	Cmd         *exec.Cmd
	obuf        bytes.Buffer
	ebuf        bytes.Buffer
	Files       []string
	Size        int64
	Index       int
	Started     bool
	Running     bool
	err         error
	verbose     bool
	debug       bool
	done        chan int
}

type ProcessSet struct {
	procs   []*Process
	verbose bool
	debug   bool
}

func NewProcess(name string, args []string) *Process {
	p := Process{
		Cmd:     exec.Command(name, args...),
		obuf:    bytes.Buffer{},
		ebuf:    bytes.Buffer{},
		Files:   []string{},
		verbose: viper.GetBool("verbose"),
		debug:   viper.GetBool("debug"),
	}
	p.Cmd.Stdout = &p.obuf
	p.Cmd.Stderr = &p.ebuf
	return &p
}

func (p *Process) Run() (string, string, error) {
	p.Running = true
	err := p.Cmd.Run()
	p.Running = false
	if err != nil {
		return "", "", err
	}
	return p.obuf.String(), p.ebuf.String(), nil
}

func NewProcessSet() *ProcessSet {
	s := ProcessSet{
		procs:   []*Process{},
		verbose: viper.GetBool("verbose"),
		debug:   viper.GetBool("debug"),
	}
	return &s
}

func NewTarsnapProcess(args []string) *Process {
	cmd := viper.GetString("tarsnap_command")
	cmdline := append(strings.Split(cmd, " "), args...)
	return NewProcess(cmdline[0], cmdline[1:])
}

func (s *ProcessSet) AddRestore(archiveName, userName, maildirName string, files []string, size int64) error {
	if s.verbose {
		log.Printf("AddRestore: %s %s %s (%d files) (%d bytes)\n", archiveName, userName, maildirName, len(files), size)
	}
	args := []string{
		"-x",
		"--fast-read",
		"-C", ExpandPath(viper.GetString("output_dir")),
		"-v", "--keyfile", ExpandPath(viper.GetString("keyfile")),
		"-f", archiveName,
	}
	args = append(args, files...)
	p := NewTarsnapProcess(args)
	p.Files = append(p.Files, files...)
	p.Size = size
	p.Index = len(s.procs)
	s.procs = append(s.procs, p)
	return nil
}

func (s *ProcessSet) Run() error {

	var processGroup sync.WaitGroup
	var progressGroup sync.WaitGroup

	limit := make(chan struct{}, PROCESS_COUNT)

	progress := !viper.GetBool("no_progress")
	done := make(chan bool)

	processGroup.Add(1)
	go func() {
		defer processGroup.Done()
		for _, proc := range s.procs {
			limit <- struct{}{}
			processGroup.Add(1)
			go func(p *Process) {
				defer processGroup.Done()
				defer func() { <-limit }()

				if p.debug {
					log.Printf("[%d] starting: %v\n", p.Index, p.Cmd)
				}
				err := p.Cmd.Start()
				if err != nil {
					p.err = fmt.Errorf("Start failed: %v", err)
					log.Printf("[%d] %v\n", p.Index, p.err)
					return
				}
				if s.verbose {
					log.Printf("[%d] running as pid %d\n", p.Index, p.Cmd.Process.Pid)
				}
				p.Started = true
				p.Running = true
				err = p.Cmd.Wait()
				p.Running = false
				if err != nil {
					p.err = fmt.Errorf("Wait failed: %v\n", err)
					log.Printf("[%d] %v\n", p.Index, p.err)
					return
				}
				p.err = nil
				if s.verbose {
					log.Printf("[%d] pid %d exited %d\n", p.Index, p.Cmd.Process.Pid, p.Cmd.ProcessState.ExitCode())
				}
			}(proc)
		}
	}()

	if progress {
		var totalSize int64
		var totalCount int
		files := []string{}
		for _, proc := range s.procs {
			totalSize += proc.Size
			totalCount += len(proc.Files)
			files = append(files, proc.Files...)
		}

		go func() {
			progressGroup.Add(1)
			defer progressGroup.Done()
			ticker := time.NewTicker(1 * time.Second)
			defer ticker.Stop()
			stopped := false
			bar := progressbar.Default(totalSize)
			for {
				select {
				case <-done:
					log.Printf("progress: read done channel\n")
					stopped = true
				case <-ticker.C:
					log.Printf("progress: tick\n")
					var count int
					var size int64
					for _, file := range files {
						targetFile := filepath.Join(ExpandPath(viper.GetString("output_dir")), file)
						if s.debug {
							log.Printf("progress: checking %s\n", targetFile)
						}
						stat, err := os.Stat(targetFile)
						if err == nil {
							count += 1
							if stat.Mode().IsRegular() {
								size += stat.Size()
							}
							if s.debug {
								log.Printf("progress: found %d %s\n", size, targetFile)
							}
						} else if os.IsNotExist(err) {
							if s.debug {
								log.Printf("progress: not found %s\n", targetFile)
							}
						} else {
							log.Fatalf("progress: Stat failed: %v", err)
						}
					}
					//log.Printf("progress: [%d of %d] %v %v\n", count, totalCount, size, totalSize)
					bar.Set64(size)
					if stopped {
						return
					}
				}
			}
		}()
	}

	if s.verbose {
		log.Printf("waiting on process group...\n")
	}
	processGroup.Wait()
	if s.verbose {
		log.Printf("all processes exited\n")
	}
	close(limit)

	if progress {
		done <- true
		if s.verbose {
			log.Printf("waiting on progress group...\n")
		}
		progressGroup.Wait()
		if s.verbose {
			log.Printf("progress group exited\n")
		}

	}
	close(done)

	return nil
}
