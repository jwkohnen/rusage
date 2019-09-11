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

package rusage

import (
	"os"
	"os/signal"
	"syscall"
	"time"
)

const (
	ErrSuccess    = 0
	ErrOther      = 1
	ErrForkExec   = 3
	ErrPermission = 126
	ErrNotFound   = 127
	ErrSignal     = 128
)

// Run runs a child command, connects STDIN, STDERR and STDOUT of this (parent)
// process to the child.  Signals defined in forwardSignals will be forwarded.
// The returned exit code will be set to either the exit code of the child
// process or to the numeric value of the signal plus 128;  if the exit code
// cannot be determined it will be 1.
//
// If executing the child command failed, err will be non-nil and code will be
// set to a non-zero value suitable for os.Exit().
//
// If dir is non-empty, the child changes into the directory before creating
// the process.
//
// If env is non-nil, it gives the environment variables for the new process in
// the form returned by os.Environ.  If it is nil, the result of os.Environ
// will be used.
//
// If starting and waiting for the child succeeded, code is 0, err is nil and r
// will contain "[...] resource usage statistics for all children of the
// calling process that have terminated and been waited for.  These statistics
// will include the resources used by grandchildren, and further removed
// descendants, if all of the intervening descendants waited on their
// terminated children" -- from: rusage(2).
//
// The list of forwarded signals is subject to change.  Expect change to that
// part of API surface.
func Run(
	cmd string,
	argv []string,
	cwd string,
	env []string,
) (
	code int,
	resourceUsage *ResourceUsage,
	err error,
) {
	defer func() { code = checkCodeRange(code) }()

	processState, elapsedTime, err := forkExecVEWait(cmd, argv, cwd, env)
	if err != nil {
		return codeFromExecErr(err), nil, err
	}

	sr := processState.SysUsage().(*syscall.Rusage)
	var (
		userTime   = time.Duration(syscall.TimevalToNsec(sr.Utime))
		systemTime = time.Duration(syscall.TimevalToNsec(sr.Stime))
		averageCPU = (userTime + systemTime).Seconds() / elapsedTime.Seconds()
	)

	resourceUsage = &ResourceUsage{
		UserTime:                     userTime,
		SystemTime:                   systemTime,
		ElapsedTime:                  elapsedTime,
		AverageCPU:                   averageCPU,
		MaxResidentSetSize:           sr.Maxrss << 10,
		MinorPageFaults:              sr.Minflt,
		MajorPageFaults:              sr.Majflt,
		InBlockIO:                    sr.Inblock,
		OutBlockIO:                   sr.Oublock,
		ContextSwitchesVoluntarily:   sr.Nvcsw,
		ContextSwitchesInvoluntarily: sr.Nivcsw,
	}

	code = codeFromProcessState(processState)

	return code, resourceUsage, nil
}

func forkExecVEWait(
	cmd string,
	argv []string,
	cwd string,
	env []string,
) (
	state *os.ProcessState,
	elapsedTime time.Duration,
	err error,
) {
	// The higher level `os/exec.Command.Start` produces an `*os.ProcessState` only in case of an "unsuccessful
	// termination." The point of this program is to gather information from the wait4 syscall's rusage struct, so we
	// need to micromanage a process through `os.StartProcess` in order to call os.Process.Wait() which yields an
	// os.ProcessState.
	//
	// Ideally this program acts like an init process: forward all signals to its
	// children and reap zombie processes.  The latter would be nice, because the wait4 syscall only accounts for
	// resource usage of all descendants if they have been wait()'ed for.  That is not implemented, yet.  This program
	// does forward relevant signals.
	//
	// The order of the defers is significant:
	// 1. The progress terminates; the PID in os.Process becomes stale.
	// 2. defer signal.Stop() stops sending to the forwarding goroutine's channel which may have some signal in its
	//    buffer; they get delivered to a stale PID, ignoring the errors.
	// 3. The forwarding goroutine's channels is closed, which terminates the goroutine.

	var forwardSignals = []os.Signal{
		syscall.SIGHUP, syscall.SIGINT, syscall.SIGCONT,
		syscall.SIGQUIT, syscall.SIGSTOP, syscall.SIGTSTP,
		syscall.SIGTERM, syscall.SIGUSR1, syscall.SIGUSR2,
	}

	ss := make(chan os.Signal, 1)
	defer close(ss)
	signal.Notify(ss, forwardSignals...)
	defer signal.Stop(ss)

	start := time.Now()
	var proc *os.Process
	proc, err = os.StartProcess(
		cmd,
		argv,
		&os.ProcAttr{
			Dir:   cwd,
			Env:   env,
			Files: []*os.File{os.Stdin, os.Stdout, os.Stderr},
		},
	)
	if err != nil {
		return nil, 0, err
	}

	go func() {
		for s := range ss {
			// #nosec G104
			_ = proc.Signal(s)
		}
	}()

	state, err = proc.Wait()
	return state, time.Since(start), err
}

func codeFromExecErr(err error) int {
	if os.IsPermission(err) {
		return ErrPermission
	}
	if os.IsNotExist(err) {
		return ErrNotFound
	}
	if errno, ok := err.(syscall.Errno); ok {
		return int(errno)
	}
	return ErrForkExec
}

func codeFromProcessState(state *os.ProcessState) int {
	// TODO (Go 1.12+): https://github.com/golang/go/issues/26539

	/*
		(*os.ProcessState).ExitCode() returns -1 when signalled
		TODO: yeah?
	*/

	ws := state.Sys().(syscall.WaitStatus)
	switch {
	case state.Success():
		return ErrSuccess
	case ws.Exited():
		return ws.ExitStatus()
	case ws.Signaled():
		return int(ErrSignal + ws.Signal())
	case ws.Stopped():
		return int(ErrSignal + ws.StopSignal())
	default:
		return ErrOther
	}
}

// checkCodeRange checks if the exit code is in the valid range 0..255. If out
// of range, it returns 255, which by convention signifies an illegal exit
// code.
//
// It is technically impossible for os.Exit to exit with an
// invalid value, but overflows might wrap over to 0; which must not
// happen.
func checkCodeRange(code int) int {
	if code < 0 || code > 255 {
		return 255
	}
	return code
}
