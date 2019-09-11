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
	"fmt"
	"os"
	"syscall"
	"testing"
	"time"
)

func TestExitCode(t *testing.T) {
	tests := []struct {
		// in
		path string
		arg  string

		// want
		code    int
		doesErr bool
	}{
		{"exit.sh", "0", 0, false},
		{"exit.sh", "1", 1, false},
		{"exit.sh", "255", 255, false},
		{"exit.sh", "256", 0, false}, // 256 is illegal, but POSIX shell's `exit` wraps it to 0.

		{"signal.sh", "SIGINT", int(ErrSignal + syscall.SIGINT), false},
		{"signal.sh", "SIGQUIT", int(ErrSignal + syscall.SIGQUIT), false},
		{"signal.sh", "SIGABRT", int(ErrSignal + syscall.SIGABRT), false},
		{"signal.sh", "SIGKILL", int(ErrSignal + syscall.SIGKILL), false},
		{"signal.sh", "SIGSEGV", int(ErrSignal + syscall.SIGSEGV), false},
		{"signal.sh", "SIGTERM", int(ErrSignal + syscall.SIGTERM), false},

		{"non-executable.txt", "--", 126, true},
		{"does-not-exist.sh", "--", 127, true},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.path+"_"+tt.arg, func(t *testing.T) {
			code, _, err := Run("./testdata/"+tt.path, []string{tt.path, tt.arg}, "", nil)
			if !tt.doesErr && err != nil {
				t.Errorf("Unexpected error: %v", err)
			}
			if tt.doesErr && err == nil {
				t.Error("Expected an error, but there was none.")
			}
			if code != tt.code {
				t.Errorf("Want code %d, got %d", tt.code, code)
			}
		})
	}
}

func TestSignalForwarding(t *testing.T) {
	// This is a hack.  `Run` temporarily modifies the signal handlers of the
	// test process itself, we signal the test process, thence Run
	// terminates while unregistering the signal handlers.  Repeat.
	//
	// It would be better to create a test main package that runs `Run` that
	// can be used as the target process for signaling.  But that means
	// this test needs to programmatically build a binary.  Ain't nobody got
	// time for that!
	for _, sig := range []syscall.Signal{
		syscall.SIGHUP,
		syscall.SIGUSR1,
		syscall.SIGUSR2,
	} {
		sig := sig
		t.Run(fmt.Sprintf("Signal_%d", sig), func(t *testing.T) {
			var got int
			done := make(chan struct{})
			go func() {
				got, _, _ = Run("./testdata/sleep.sh", []string{"sleep", "5"}, "", nil)
				close(done)
			}()
			time.Sleep(20e6)
			proc, err := os.FindProcess(os.Getpid())
			if err != nil {
				t.Fatal(err)
			}
			err = proc.Signal(sig)
			if err != nil {
				t.Fatal(err)
			}
			select {
			case <-done:
			case <-time.After(5e9):
				t.Fatal("timeout")
			}
			if want := int(128 + sig); want != got {
				t.Errorf("Want code %d, got %d", want, got)
			}
		})
	}
}
