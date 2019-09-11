//    This file is part of rusage.
//
//    rusage is free software: you can redistribute it and/or modify it under
//    the terms of the GNU General Public License as published by the Free
//    Software Foundation, either version 3 of the License, or (at your option)
//    any later version.
//
//    rusage is distributed in the hope that it will be useful, but WITHOUT ANY
//    WARRANTY; without even the implied warranty of MERCHANTABILITY or FITNESS
//    FOR A PARTICULAR PURPOSE.  See the GNU General Public License for more
//    details.
//
//    You should have received a copy of the GNU General Public License along
//    with rusage.  If not, see <http://www.gnu.org/licenses/>.

// Rusage is a command that does what /usr/bin/time (not the shell builtin)
// does on most systems.  It is not meant as a replacement, but for debugging.
package main

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"os/exec"
	"runtime"

	"github.com/jwkohnen/rusage"
)

func main() {
	if goos := runtime.GOOS; goos != "linux" {
		_, _ = fmt.Fprintf(os.Stderr, "Running %s on %s yields undefined results or may crash.\n", os.Args[0], goos)
	}
	if len(os.Args) < 2 {
		log.Fatal("Need more args!")
	}
	cmd, err := exec.LookPath(os.Args[1])
	if err != nil {
		log.Fatal(err)
	}

	code, resourceUsage, err := rusage.Run(cmd, os.Args[1:], "", nil)
	if err != nil {
		log.Fatal(err)
	}
	enc := json.NewEncoder(os.Stderr)
	enc.SetIndent("", "\t")
	enc.SetEscapeHTML(false)
	err = enc.Encode(resourceUsage)
	if err != nil {
		panic(err)
	}
	os.Exit(code)
}
