# rusage: a process wrapper that reports success/failure and resource usage to a prometheus pushgateway

[![GNU Public License v3.0](https://img.shields.io/badge/license-GPLv3-blue.svg)](https://www.gnu.org/licenses/gpl.txt)
[![GoDoc](https://godoc.org/github.com/jwkohnen/rusage?status.svg)](https://godoc.org/github.com/jwkohnen/rusage)

Package rusage provides a simple process wrapper like GNU time, that reports
resource usage metrics, most importantly CPU usage and maximum resident set
size, start / end time and success / failure to a prometheus pushgateway. It is
meant for ephemeral or batch jobs that are otherwise hard for prometheus to
scrape. In the regular use case you can use the `prometheus-rusage-pusher` as a
prefix in your command invocation line or as a Docker entrypoint and the aspect
of metrics gathering is covered so you can keep the code of your workload free
of prometheus details.


## State

This is WIP; this code doesn't work yet, tests may fail or nothing even compiles!

## Links

https://prometheus.io/docs/concepts/jobs_instances/

https://www.robustperception.io/idempotent-cron-jobs-are-operable-cron-jobs

## License

License under GNU Public License v3.0
