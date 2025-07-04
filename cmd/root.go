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
	"os"

	"github.com/spf13/cobra"
)

var cfgFile string

var rootCmd = &cobra.Command{
	Version: "0.0.8",
	Use:     "tarsnap-maildir-restore",
	Short:   "restore tarsnap maildir backup",
	Long: `
Daily tarsnap maildir backups execute on mailservers.
Given a date, the command lists the files in the backup, then divides
the data to be restored into multiple sets and starts a number of restore
commands running in parallel.
`,
}

func Execute() {
	err := rootCmd.Execute()
	if err != nil {
		os.Exit(1)
	}
}
func init() {
	cobra.OnInitialize(InitConfig)
	OptionString("logfile", "l", "", "log filename")
	OptionString("config", "c", "", "config file")
	OptionSwitch("debug", "", "produce debug output")
	OptionSwitch("verbose", "v", "increase verbosity")
	OptionSwitch("json", "j", "output json")
	OptionSwitch("no-progress", "P", "suppress progress bar display")
	OptionString("keyfile", "k", "", "tarsnap key file")
	OptionString("archive", "a", "", "archive base name YYYY-MM-DD.hostname")
	OptionString("user", "u", ".*", "username select filter (regex)")
	OptionString("maildir", "m", ".*", "maildir select filter (regex)")
	OptionString("output-dir", "O", "./restore", "restore destination directory")
	OptionString("metadata-dir", "M", "", "preloaded metadata directory")
	OptionString("tarsnap-command", "T", "/usr/local/bin/tarsnap", "tarsnap command")
}
