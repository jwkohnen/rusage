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
	"errors"
	"flag"
	"os"
	"os/exec"

	"github.com/prometheus/common/model"

	"github.com/jwkohnen/rusage/pusher"
)

var (
	errNoCommandSpecified = errors.New("no command specified")
)

type config struct {
	path      string
	argv      []string
	grouping  *pusher.Labels
	labels    *pusher.Labels
	gateway   string
	namespace string
}

func configure() (cfg *config, err error) {
	cfg = &config{
		grouping:  pusher.NewLabels(model.LabelSet{"job": "rusage"}),
		labels:    defaultLabels(),
		gateway:   "http://pushgateway:9091",
		namespace: "TODO_THE_NAMESPACE",
	}
	flag.Var(
		cfg.grouping,
		"grouping",
		"Labels used for grouping of metrics. Must be unique per job.",
	)
	flag.Var(
		cfg.labels,
		"label",
		"Labels used for additional information, not for grouping.",
	)
	flag.StringVar(
		&cfg.gateway,
		"gateway",
		cfg.gateway,
		`URL to the push gateway, w/o "metrics/jobs/..." part`,
	)
	flag.StringVar(
		&cfg.namespace,
		"namespace",
		cfg.namespace,
		"Namespace to prefix the metric names with.",
	)
	flag.Parse()
	if flag.NArg() < 1 {
		return nil, errNoCommandSpecified
	}

	path, err := exec.LookPath(flag.Arg(0))
	if err != nil {
		return nil, err
	}

	cfg.path = path
	cfg.argv = flag.Args()

	return cfg, nil
}

func defaultLabels() *pusher.Labels {
	var labels *pusher.Labels
	hostname, err := os.Hostname()
	if err == nil {
		// labels = pusher.Init(model.LabelSet{model.LabelName("hostname"): model.LabelValue(hostname)})
		labels = pusher.NewLabels(model.LabelSet{"hostname": model.LabelValue(hostname)})
	}

	return labels
}
