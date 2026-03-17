BINARY=kush

.PHONY: all build build-debug clean

all: build

build:
	go build -o $(BINARY) .

build-debug:
	go build -gcflags "all=-N -l" -o $(BINARY).debug .

clean:
	rm -f $(BINARY) $(BINARY).debug
