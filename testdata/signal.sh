#!/bin/sh
set -eu

/bin/kill --signal "$1" $$
