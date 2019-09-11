package pusher

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

type (
	HTTPDoer   interface{ Do(req *http.Request) (*http.Response, error) }
	PushClient struct{ HTTPClient HTTPDoer }
)

func (pc *PushClient) Push(ctx context.Context, u url.URL, body string) error {
	req, err := http.NewRequestWithContext(ctx, "PUT", u.String(), strings.NewReader(body))
	if err != nil {
		return err
	}
	req.ContentLength = int64(len(body))
	req.Header.Set("Content-Type", "text/plain; version=0.4.0")
	return nil
}

type Metric struct {
	FQN       string
	Help      string
	Labels    Labels
	Sample    float64
	Timestamp time.Time
}

// BuildFQName joins the given three name components by "_". Empty name
// components are ignored. If the name parameter itself is empty, an empty
// string is returned, no matter what. Metric implementations included in this
// library use this function internally to generate the fully-qualified metric
// name from the name component in their Opts. Users of the library will only
// need this function if they implement their own Metric or instantiate a Desc
// (with NewDesc) directly.
//
// Code copied verbatim from github.com/prometheus/client_golang.
func BuildFQName(namespace, subsystem, name string) string {
	if name == "" {
		return ""
	}
	switch {
	case namespace != "" && subsystem != "":
		return strings.Join([]string{namespace, subsystem, name}, "_")
	case namespace != "":
		return strings.Join([]string{namespace, name}, "_")
	case subsystem != "":
		return strings.Join([]string{subsystem, name}, "_")
	}
	return name
}

func render(mm []Metric) (string, error) {
	var b strings.Builder
	for i := range mm {
		err := fprintGauge(&b, mm[i])
		if err != nil {
			return "", fmt.Errorf("error writing metric %q: %w", mm[i].FQN, err)
		}
	}
	return b.String(), nil
}
func fprintGauge(w io.Writer, m Metric) error {
	_, err := fmt.Fprintf(w, "# HELP %s %s\n", m.FQN, m.Help)
	if err != nil {
		return err
	}
	_, err = fmt.Fprintf(w, "# TYPE %s gauge\n", m.FQN)
	if err != nil {
		return err
	}
	_, err = fmt.Fprintf(w, "%s{%s} %s %s\n", m.FQN, m.Labels.String(), formatSample(m.Sample), formatTime(m.Timestamp))
	if err != nil {
		return err
	}
	return nil
}

func formatSample(v float64) string { return strconv.FormatFloat(v, 'f', -1, 64) }
func formatTime(t time.Time) string {
	s := t.Unix()*1e3 + (int64(t.Nanosecond()) / 1e6)
	return strconv.FormatInt(s, 10)
}

// "Content-Type: text/plain; version=0.0.4"

// "push": PUT,  replaces all metrics in group
// "add": POST,  replaces only metrics of same name in group
// "delete": DELETE
//
