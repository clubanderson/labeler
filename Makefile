SOURCES := $(shell find pkg -name '*.go')

build:
	go build -o labeler $(SOURCES)

deploy:
	sudo cp labeler /usr/local/bin

run:
	go run labeler.go

compile:
	echo "Compiling for every OS and Platform"
	GOOS=linux GOARCH=arm go build -o $(SOURCES)
	GOOS=linux GOARCH=arm64 go build -o $(SOURCES)
	GOOS=freebsd GOARCH=386 go build -o $(SOURCES)

