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
	# git -C ${GOPATH}/src/github.com/ory/hydra/ checkout --track origin/v1.0.0-beta.4

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
