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

package main

import (
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"time"

	"github.com/jwkohnen/rusage"
	"github.com/jwkohnen/rusage/pusher"
)

func main() {
	if goos := runtime.GOOS; goos != "linux" {
		_, _ = fmt.Fprintf(os.Stderr, "Running %s on %s yields undefined results or may crash.", os.Args[0], goos)
	}
	cfg, err := configure()
	if err != nil {
		exit(errCode(err), err)
		panic("the actual fuck?") // TODO
	}

	code := runAndPush(cfg)

	os.Exit(code)
}

func runAndPush(cfg *config) int {
	push := pusher.New(pusher.Config{
		Gateway:   cfg.gateway,
		Grouping:  cfg.grouping.LabelSet(),
		Labels:    cfg.labels.LabelSet(),
		Namespace: cfg.namespace,
	})

	start := time.Now()
	err := push.Start(start)
	if err != nil {
		// TODO
		panic(err)
	}
	code, usage, err := rusage.Run(
		cfg.path,
		cfg.argv,
		/* stay in cwd */ "",
		/* inherit environ */ nil,
	)
	if err != nil {
		exit(code, err)
	}
	_ = usage // TODO
	/*	err = push.End(start, code, usage)
		if err != nil {
			// TODO
			panic(err)
		}
	*/
	return code
}

func exit(code int, err error) {
	_, _ = fmt.Fprintf(os.Stderr, "rusage: %v\n", err)
	os.Exit(code)
}

func errCode(err error) int {
	if os.IsNotExist(err) {
		return 127
	}
	if os.IsPermission(err) {
		return 128
	}
	if rec, ok := err.(*exec.Error); ok {
		return errCode(rec.Err)
	}
	if err == errNoCommandSpecified {
		return 80
	}
	return 1
}
