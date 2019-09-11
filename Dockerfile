FROM	golang:1.13 as build
ENV	GO111MODULES=on CGO_ENABLED=0
WORKDIR	/outofgopath
COPY	go.mod go.sum ./
RUN	go mod download \
&&	go mod verify
COPY	*.go ./
COPY	cmd cmd
COPY	pusher pusher
COPY	testdata testdata
RUN	go install --mod=readonly -v ./...
RUN	go test -v ./... || true

FROM	scratch
COPY 	--from=build /go/bin/* /usr/local/bin/
ENTRYPOINT	[ "/usr/local/bin/prometheus-rusage-pusher" ]
CMD	[ "--help" ]
