package cmd

import (
	"bufio"
	"bytes"
	"fmt"
	"github.com/spf13/viper"
	"os"
	"os/exec"
	"sync"
)

type Process struct {
	CommandLine string
	Cmd         *exec.Cmd
	obuf        bytes.Buffer
	ebuf        bytes.Buffer
	Patterns    []string
	Blocks      int
	Started     bool
	Running     bool
	err         error
	done        chan int
}

type ProcessSet struct {
	procs []*Process
}

func NewProcess(name string, args []string) *Process {
	p := Process{
		Cmd:      exec.Command(name, args...),
		obuf:     bytes.Buffer{},
		ebuf:     bytes.Buffer{},
		Patterns: []string{},
	}
	p.Cmd.Stdout = bufio.NewWriter(&p.obuf)
	p.Cmd.Stderr = bufio.NewWriter(&p.ebuf)
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
		procs: []*Process{},
	}
	return &s
}

func (s *ProcessSet) AddRestore(archiveName string, patterns []string) error {
	//log.Printf("AddRestore: %s %+v\n", archiveName, patterns)
	args := []string{"-x", "--fast-read", "-v", "--keyfile", viper.GetString("keyfile"), "-f", archiveName}
	for _, pattern := range patterns {
		args = append(args, pattern)
	}
	p := NewProcess("tarsnap", args)
	p.Patterns = patterns
	s.procs = append(s.procs, p)
	return nil
}

func (s *ProcessSet) Wait() error {

	var wg sync.WaitGroup

	for i, proc := range s.procs {

		err := proc.Cmd.Start()
		if err != nil {
			return err
		}
		proc.Started = true
		proc.Running = true

		wg.Add(1)
		go func(i int, p *Process) {
			fmt.Fprintf(os.Stderr, "[%d] goprocess started %v\n", i, p.Cmd.Args[:8])
			defer wg.Done()
			fmt.Fprintf(os.Stderr, "[%d] awaiting process: %d\n", i, p.Cmd.Process.Pid)
			err := p.Cmd.Wait()
			p.err = err
			p.Running = false
			if err != nil {
				fmt.Fprintf(os.Stderr, "[%d] Wait failed: %v\n", i, err)
			} else {
				fmt.Fprintf(os.Stderr, "[%d] process exited: %d\n", i, p.Cmd.Process.Pid)
			}
		}(i, proc)
	}

	if viper.GetBool("progress") {
	}

	fmt.Printf("waiting on waitgroup...\n")
	wg.Wait()

	fmt.Printf("all processes exited\n")
	return nil
}
