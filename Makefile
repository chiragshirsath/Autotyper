.PHONY: all build run stop clean

all: build

build:
	go build -ldflags="-s -w" -o server ./cmd/server
	go build -ldflags="-s -w" -o stop ./cmd/stop
	chmod +x server stop

run: build
	./server

stop:
	./stop

clean:
	rm -f server stop
