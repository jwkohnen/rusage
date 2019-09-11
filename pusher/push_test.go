package pusher

import (
	"context"
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"math"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/prometheus/client_golang/api"
	prometheus "github.com/prometheus/client_golang/api/prometheus/v1"
	"github.com/prometheus/common/model"
	"golang.org/x/sync/errgroup"

	"github.com/jwkohnen/rusage"
)

const (
	pushPort           = "19091"
	pushAddr           = "localhost:" + pushPort
	pushReadinessProbe = "http://" + pushAddr + "/#/status"

	promPort           = "19090"
	promAddr           = "localhost:" + promPort
	promURL            = "http://" + promAddr
	promReadinessProbe = promURL + "/-/healthy"
)

func TestStart(t *testing.T) {
	t.Parallel()

	const (
		metricStartTime      = "TODO_the_start_time"
		metricCompletionTime = "TODO_the_termination_time"
		metricSuccessTime    = "TODO_the_success_time"
	)
	const (
		testStartPushJob = "test_start_push_job"
		testNoSuccessJob = "test_no_success_job"
	)
	tests := map[string][]struct {
		// group labels and extra label used for pushing
		group, labels model.LabelSet

		// labels that select metrics in query
		selector model.LabelSet

		// exit code of process
		code int

		hasSuccessTimeMetric bool

		// a mock value used for metrics
		markerValue int64
	}{
		testStartPushJob: {
			{
				group:                model.LabelSet{"job": testStartPushJob, "test": "start"},
				labels:               model.LabelSet{"run": "one"},
				selector:             model.LabelSet{"run": "one"},
				code:                 0,
				hasSuccessTimeMetric: true,
				markerValue:          1,
			},
			{
				group:                model.LabelSet{"job": testStartPushJob, "test": "start"},
				labels:               model.LabelSet{"two": "two"},
				selector:             model.LabelSet{"job": testStartPushJob, "test": "start"},
				code:                 0,
				hasSuccessTimeMetric: true,
				markerValue:          2,
			},
		},
		testNoSuccessJob: {
			{
				group:                model.LabelSet{"job": testNoSuccessJob, "test": "noSuccess"},
				labels:               model.LabelSet{"selector": testNoSuccessJob},
				selector:             model.LabelSet{"selector": testNoSuccessJob},
				code:                 1,
				hasSuccessTimeMetric: false,
				markerValue:          3,
			},
		},
	}

	testRunLabelValue := model.LabelValue(fmt.Sprintf("test_run_%v", time.Now().Format(time.RFC3339Nano)))
	testRunLabel := model.LabelSet{"test_run": testRunLabelValue}

	for name, tt := range tests {
		name, tt := name, tt
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			for _, job := range tt {
				group := job.group.Merge(testRunLabel)
				pusher := New(Config{
					Gateway:   pushAddr,
					Namespace: "TODO_namespace",
					Grouping:  group,
					Labels:    job.labels,
				})

				beforeStart := time.Now()
				err := pusher.Start(beforeStart)
				if err != nil {
					t.Fatal(err)
				}
				betweenStartAndEnd := time.Now()
				err = pusher.End(betweenStartAndEnd, job.code, &rusage.ResourceUsage{
					UserTime:                     time.Duration(job.markerValue),
					SystemTime:                   time.Duration(job.markerValue),
					ElapsedTime:                  betweenStartAndEnd.Sub(beforeStart),
					AverageCPU:                   float64(job.markerValue),
					MaxResidentSetSize:           job.markerValue,
					MinorPageFaults:              job.markerValue,
					MajorPageFaults:              job.markerValue,
					InBlockIO:                    job.markerValue,
					OutBlockIO:                   job.markerValue,
					ContextSwitchesVoluntarily:   job.markerValue,
					ContextSwitchesInvoluntarily: job.markerValue,
				})
				if err != nil {
					t.Fatal(err)
				}
				afterEnd := time.Now()

				// test time based metrics
			ForMetric:
				for metric, want := range map[string]struct {
					query    string
					oldest   time.Time
					youngest time.Time
				}{
					metricStartTime:      {metricStartTime, beforeStart, betweenStartAndEnd},
					metricCompletionTime: {metricCompletionTime, betweenStartAndEnd, afterEnd},
					metricSuccessTime:    {metricSuccessTime, betweenStartAndEnd, afterEnd},
				} {
					var (
						vector   model.Vector
						n        int
						deadline = time.Now().Add(10e9)
					)

					selector := job.selector.Merge(testRunLabel)
				ForRetryQuery:
					for time.Now().Before(deadline) {
						vector = queryVector(t, metric, selector)
						n = len(vector)
						if n > 0 {
							break ForRetryQuery
						}
						time.Sleep(20e6)
					}
					//noinspection GoNilness
					t.Logf(`"%s%s=%s"`, metric, selector, vector.String())

					if metric == metricSuccessTime && !job.hasSuccessTimeMetric {
						if n != 0 {
							t.Errorf("There should be no %q metric, but there is: %v", metric, vector)
						}
						continue ForMetric
					}
					if n != 1 {
						t.Fatalf("Want 1 sample, got %d", n)
					}

					//noinspection GoNilness
					ts := timeFromSample(vector[0])
					if ts.Before(want.oldest) || ts.After(want.youngest) {
						t.Errorf("Timestamp of metric %q is out of bounds !(%v <= %v <= %v)",
							metric+job.labels.Merge(job.group).String(),
							want.oldest,
							ts,
							want.youngest,
						)
					}

				}
			}
		})
	}
}

func timeFromSample(sample *model.Sample) time.Time {
	sec, frac := math.Modf(float64(sample.Value))
	return time.Unix(int64(sec), int64(frac*1e9))
}

func TestMain(m *testing.M) {
	var (
		forceBuild  bool
		keepWaiting bool
	)
	flag.BoolVar(&forceBuild, "force-build", forceBuild,
		"rebuilds prometheus-server and pushgateway binaries",
	)
	flag.BoolVar(&keepWaiting, "keep-waiting", keepWaiting,
		"wait after tests have finished",
	)
	flag.Parse()

	gobin, err := filepath.Abs(filepath.Join("testdata", "generated", "bin"))
	if err != nil {
		panic(err)
	}
	workdir, err := ioutil.TempDir("", "rusage_pusher_test")
	if err != nil {
		panic(err)
	}
	var (
		pushgateway                       = filepath.Join(gobin, "pushgateway")
		promServer                        = filepath.Join(gobin, "prometheus")
		ctx, cancel                       = context.WithTimeout(context.Background(), 2*time.Minute)
		code                              int
		pushGatewayDone                   <-chan struct{}
		promServerDone                    <-chan struct{}
		shutdown                          = make(chan struct{})
		buildGroup, startGroup, waitGroup errgroup.Group
	)

	// Will never actually be called, but it stands here for completeness.
	defer cancel()

	buildGroup.Go(func() error { return buildPrometheusBinary(ctx, forceBuild, workdir, gobin, pushgateway) })
	buildGroup.Go(func() error { return buildPrometheusBinary(ctx, forceBuild, workdir, gobin, promServer) })
	err = buildGroup.Wait()
	if err != nil {
		goto Shutdown
	}
	startGroup.Go(func() error { var e error; e, pushGatewayDone = startPushGateway(ctx, workdir, shutdown, pushgateway); return e })
	startGroup.Go(func() error { var e error; e, promServerDone = startPromServer(ctx, workdir, shutdown, promServer); return e })
	err = startGroup.Wait()
	if err != nil {
		goto Shutdown
	}
	waitGroup.Go(func() error { return waitReady(ctx, pushReadinessProbe) })
	waitGroup.Go(func() error { return waitReady(ctx, promReadinessProbe) })
	err = waitGroup.Wait()
	if err != nil {
		goto Shutdown
	}

	code = m.Run()

	if keepWaiting {
		time.Sleep(5 * time.Minute)
	}

Shutdown:
	close(shutdown)

	if err != nil {
		log.Printf("Error starting up background services: %v", err)
		if code == 0 {
			code = 1
		}
	}

	if pushGatewayDone != nil {
		<-pushGatewayDone
	}
	if promServerDone != nil {
		<-promServerDone
	}
	os.Exit(code)
}

func waitReady(ctx context.Context, readinessProbe string) error {
ForRetry:
	for {
		ready := func() bool {
			reqCtx, cancel := context.WithTimeout(ctx, 50e6)
			defer cancel()

			req, err := http.NewRequestWithContext(reqCtx, "GET", readinessProbe, nil)
			if err != nil {
				panic(err)
			}
			response, err := http.DefaultClient.Do(req)
			if err != nil {
				return false
			}
			_ = response.Body.Close()
			return response.StatusCode == http.StatusOK
		}()
		if ready {
			return nil
		}

		select {
		case <-time.After(20e6):
			continue ForRetry
		case <-ctx.Done():
			return ctx.Err()
		}
	}
}

func queryVector(t *testing.T, query string, selector model.LabelSet) model.Vector {
	t.Helper()
	query += selector.String()
	deadline := time.Now().Add(15 * time.Second)

ForRetry:
	for time.Now().Before(deadline) {
		var (
			v    model.Value
			warn api.Warnings
			err  error
		)
		func() {
			ctx, cancel := context.WithTimeout(context.Background(), 100e6)
			defer cancel()
			v, warn, err = promAPIClient().Query(ctx, query, time.Now())
		}()
		if err != nil {
			t.Logf("Query produced error: %v", err)
			continue ForRetry
		}
		if len(warn) > 0 {
			t.Logf("Query produced warnings: %v", warn)
		}
		vec, ok := v.(model.Vector)
		if !ok {
			t.Fatalf("expected a vector, got %v: %v", v.Type(), v.String())
		}

		if len(vec) > 0 {
			return vec
		}
		time.Sleep(20e6)
	}
	t.Logf("No data for query: %s", query)
	return nil
}

func promAPIClient() prometheus.API {
	client, err := api.NewClient(api.Config{Address: promURL})
	if err != nil {
		panic(err)
	}
	return prometheus.NewAPI(client)
}

func buildPrometheusBinary(ctx context.Context, rebuildBinary bool, workdir, gobin, binary string) error {
	start := time.Now()
	stat, err := os.Stat(binary)
	if err != nil && !os.IsNotExist(err) {
		return err
	}
	if err == nil && !rebuildBinary && stat.Mode()&os.ModePerm == 0755 {
		return nil
	}

	URL, ref, pkg, vendor := mapBinaryToGoModuleAndPackageCoordinates(binary)
	err = gitClone(ctx, URL, ref, workdir)
	if err != nil {
		return err
	}
	workdir = filepath.Join(workdir, filepath.Base(binary))
	err = goInstall(ctx, workdir, pkg, gobin, vendor)
	_, _ = fmt.Fprintf(os.Stderr, "Rebuilding %q took %s.\n", binary, time.Since(start))
	return err
}

func mapBinaryToGoModuleAndPackageCoordinates(binary string) (URL, ref, pkg string, vendor bool) {
	switch {
	case filepath.Base(binary) == "pushgateway":
		return "https://github.com/prometheus/pushgateway.git",
			"v0.9.1",
			".",
			false
	case filepath.Base(binary) == "prometheus":
		return "https://github.com/prometheus/prometheus.git",
			"v2.12.0",
			"./cmd/prometheus",
			true
	default:
		panic("wat?")
	}
}

func gitClone(ctx context.Context, URL, ref, dir string) error {
	cmd := exec.CommandContext(ctx, "git", "clone", "--branch="+ref, "--depth=1", URL)
	cmd.Dir = dir
	// cmd.Stdout = os.Stderr
	// cmd.Stderr = os.Stderr
	return cmd.Run()
}

func goInstall(ctx context.Context, dir, pkg, gobin string, vendor bool) error {
	var argv = []string{"install", pkg}
	if vendor {
		argv = []string{"install", "--mod=vendor", pkg}
	}
	cmd := exec.CommandContext(ctx, "go", argv...)
	cmd.Dir = dir
	cmd.Env = makeEnv(os.Environ(), gobin)
	// cmd.Stdout = os.Stderr
	// cmd.Stderr = os.Stderr
	return cmd.Run()
}

func makeEnv(env []string, gobin string) []string {
	result := make([]string, 0, len(env)+2)
	for _, e := range env {
		switch {
		case strings.HasPrefix(e, "GOBIN="):
			continue
		case strings.HasPrefix(e, "GO111MODULE="):
			continue
		default:
			result = append(result, e)
		}
	}
	return append(result, "GO111MODULE=on", "GOBIN="+gobin)
}

func startPushGateway(ctx context.Context, workdir string, stop <-chan struct{}, exe string) (error, <-chan struct{}) {
	args := []string{"--web.listen-address=" + pushAddr}
	return startService(ctx, workdir, stop, exe, args)
}

func startPromServer(ctx context.Context, workdir string, stop <-chan struct{}, exe string) (error, <-chan struct{}) {
	var (
		cfgFile = filepath.Join(workdir, "prometheus.yaml")
		dataDir = filepath.Join(workdir, "data")
		argv    = []string{
			"--config.file=" + cfgFile,
			"--storage.tsdb.path=" + dataDir,
			"--web.listen-address=" + promAddr,
		}
		config = fmt.Sprintf(
			"scrape_configs:\n"+
				"  - job_name: pushgateway\n"+
				"    scrape_interval: 200ms\n"+
				"    honor_labels: true\n"+
				"    static_configs:\n"+
				"    - targets: [ %q ]\n", pushAddr,
		)
	)

	err := ioutil.WriteFile(cfgFile, []byte(config), 0666)
	if err != nil {
		return err, nil
	}

	return startService(ctx, workdir, stop, exe, argv)
}

func startService(
	ctx context.Context,
	workdir string,
	stop <-chan struct{},
	exe string,
	args []string,
) (
	error,
	<-chan struct{},
) {
	cmd := exec.CommandContext(ctx, exe, args...)
	cmd.Dir = workdir

	err := cmd.Start()
	if err != nil {
		return err, nil
	}

	done := make(chan struct{})
	go func() {
		defer close(done)
		select {
		case <-stop:
		case <-ctx.Done():
		}
		_ = cmd.Process.Signal(os.Interrupt)
		_ = cmd.Wait()
	}()

	return nil, done
}
