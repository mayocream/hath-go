PROG=bin/hath

COMMIT_HASH=$(shell git rev-parse --short HEAD || echo "GitNotFound")

BUILD_DATE=$(shell date '+%Y-%m-%d %H:%M:%S')

CFLAGS = -ldflags "-s -w -X \"main.BuildVersion=${COMMIT_HASH}\" -X \"main.BuildDate=$(BUILD_DATE)\""

SRCS=./cli/hath/*.go

all:
	if [ ! -d "./bin/" ]; then \
		mkdir bin; \
	fi
	CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build $(CFLAGS) -o $(PROG) $(SRCS)

race:
	if [ ! -d "./bin/" ]; then \
    	mkdir bin; \
    fi
	go build $(CFLAGS) -race -o $(PROG) $(SRCS)

clean:
	rm -rf ./bin