ifeq ($(OS), Windows_NT)
	BIN_FILENAME  := my-grpc-server.exe
else
	BIN_FILENAME  := my-grpc-server
endif

.PHONY: tidy
tidy:
	go mod tidy


.PHONY: clean
clean:
ifeq ($(OS), Windows_NT)
	rm -rf bin	
else
	rm -fR ./bin
endif


.PHONY: build
build: clean
	go build -o ./bin/${BIN_FILENAME} ./cmd


.PHONY: execute
execute: clean build
	./bin/${BIN_FILENAME}