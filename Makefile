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
	go get -v -d -t ./...
	git -C ${GOPATH}/src/github.com/segmentio/analytics-go/ checkout --track origin/v3.0
	git -C ${GOPATH}/src/github.com/ory/metrics-middleware checkout db3300574e48a229d5ddb1e30ea4adfd139d493a

test:
	go test ./...

test-race:
	go test -race ./...

test-coverage:
	go test -coverprofile=coverage.out -covermode=count -coverpkg=$(shell go list ./... | grep -v '/vendor/' | paste -sd, -) ./...

vet:
	@if [ "`go vet ./... | tee /dev/stderr`" ]; then \
		echo "^ go vet errors!" && echo && exit 1; \
	fi

build:
	go build -ldflags="-w -s" .

build-dev:
	go build .
