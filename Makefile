SOURCES := $(shell find pkg -name '*.go')

build:
	go build -o labeler $(SOURCES)

install:
	sudo cp labeler /usr/local/bin

run:
	go run labeler.go

compile:
	echo "Compiling for every OS and Platform"
	GOOS=linux GOARCH=arm go build -o bin/labeler-linux-arm $(SOURCES)
	GOOS=linux GOARCH=arm64 go build -o bin/labeler-linux-arm64 $(SOURCES)
	GOOS=linux GOARCH=386 go build -o bin/labeler-linux-386 $(SOURCES)
	GOOS=windows GOARCH=386 go build -o bin/labeler-windows-386 $(SOURCES)
	GOOS=freebsd GOARCH=386 go build -o bin/labeler-freebsd-386 $(SOURCES)

all: build install