check: get fmt vet lint test test-race

fmt:
	@for d in $(DIRS) ; do \
		if [ "`gofmt -s -l $$d/*.go | tee /dev/stderr`" ]; then \
			echo "^ improperly formatted go files" && echo && exit 1; \
		fi \
	done

lint:
	@if [ "`gometalinter --cyclo-over=15 --deadline=5m ./... | tee /dev/stderr`" ]; then \
		echo "^ gometalinter errors!" && echo && exit 1; \
	fi

get:
	go get -v -d -u -t ./...
	git -C ${GOPATH}/src/github.com/square/go-jose/ checkout --track origin/v2
	go get -d -v -u github.com/square/go-jose
	git -C ${GOPATH}/src/github.com/segmentio/analytics-go/ checkout --track origin/v3.0
	git -C ${GOPATH}/src/github.com/ory/hydra/ checkout --track origin/0.11

test:
	go test ./...

test-race:
	go test -race ./...

vet:
	@if [ "`go vet ./... | tee /dev/stderr`" ]; then \
		echo "^ go vet errors!" && echo && exit 1; \
	fi

build:
	go build -ldflags="-w -s" .

build-dev:
	go build .
