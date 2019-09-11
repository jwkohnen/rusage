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
	"reflect"
	"time"
)

// ResourceUsage contains resource usage values of a process. See getrusage(2)
// for details.  Resource usage metrics that are not provided by modern Linux
// kernels are not declared.
type ResourceUsage struct {
	// UserTime is the total amount of time spent executing in user mode
	UserTime time.Duration `rusage_metric:"user_time"`

	// SystemTime is the total amount of time spent executing in kernel mode
	SystemTime time.Duration `rusage_metric:"system_time"`

	// Elapsed real time is the time that passed during the execution of
	// the process.  It uses the monotonic process timer that is resilient
	// against local clock skew due to e.g. leap-second, clock smearing and
	// the NTP clock slew discipline.
	ElapsedTime time.Duration `rusage_metric:"elapsed_time"`

	// AverageCPU is (UTime + STime) / ElapsedTime
	AverageCPU float64 `rusage_metric:"average_cpu"`

	// MaxResidentSetSize is the maximum resident set size used (in bytes)
	MaxResidentSetSize int64 `rusage_metric:"max_resident_set_size"`

	// MinorPageFaults is the number of page faults serviced without any
	// I/O activity; here I/O activity is avoided by “reclaiming” a page
	// frame from the list of pages awaiting reallocation
	MinorPageFaults int64 `rusage_metric:"minor_page_faults"`

	// MajorPageFaults is the number of page faults serviced that required
	// I/O activity
	MajorPageFaults int64 `rusage_metric:"major_page_faults"`

	// InBlockIO is the  number of times the filesystem had to perform
	// input
	InBlockIO int64 `rusage_metric:"in_block_io"`

	// OutBlockIO is the number of times the filesystem had to perform
	// output
	OutBlockIO int64 `rusage_metric:"out_block_io"`

	// ContextSwitchesVoluntarily is the number of times a context switch
	// resulted due to a process voluntarily giving up the processor before
	// its time slice was completed (usually to await availability of a
	// resource)
	ContextSwitchesVoluntarily int64 `rusage_metric:"context_switches_voluntary"`

	// ContextSwitchesInvoluntarily is the number of times a context switch
	// resulted due to a higher priority process becoming runnable or
	// because the current process exceeded its time slice
	ContextSwitchesInvoluntarily int64 `rusage_metric:"context_switches_involuntary"`
}

var (
	metricNames = getMetricNames()
	metricDoc   = map[string]string{
		"user_time":   "UserTime is the total amount of time spent executing in user mode",
		"system_time": "SystemTime is the total amount of time spent executing in kernel mode",
		"elapsed_time": "Elapsed real time is the time that passed during the execution of the process.  It uses the " +
			"monotonic process timer that is resilient against local clock skew due to e.g. leap-second, clock " +
			"smearing and the NTP clock slew discipline.",
		"average_cpu":           "AverageCPU is (UTime + STime) / ElapsedTime",
		"max_resident_set_size": "MaxResidentSetSize is the maximum resident set size used (in bytes)",
		"minor_page_faults": "MinorPageFaults is the number of page faults serviced without any I/O activity; here I/O " +
			"activity is avoided by “reclaiming” a page frame from the list of pages awaiting reallocation",
		"major_page_faults": "MajorPageFaults is the number of page faults serviced that required I/O activity",
		"in_block_io":       "InBlockIO is the  number of times the filesystem had to perform input",
		"out_block_io":      "OutBlockIO is the number of times the filesystem had to perform output",
		"context_switches_voluntary": "ContextSwitchesVoluntarily is the number of times a context switch resulted " +
			"due to a process voluntarily giving up the processor before its time slice was completed (usually to " +
			"await availability of a resource)",
		"context_switches_involuntary": "ContextSwitchesInvoluntarily is the number of times a context switch " +
			"resulted due to a higher priority process becoming runnable or because the current process exceeded its " +
			"time slice",
	}
)

func getMetricNames() []string {
	const metricNameStructKey = "rusage_metric"
	t := reflect.TypeOf(ResourceUsage{})
	n := t.NumField()
	mm := make([]string, n)
	for i := 0; i < n; i++ {
		f := t.Field(i)
		m, ok := f.Tag.Lookup(metricNameStructKey)
		if !ok {
			panic(fmt.Errorf("struct tag %q missing on field %v", metricNameStructKey, f))
		}
		mm[i] = m
	}
	return mm
}
