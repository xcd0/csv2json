BIN           := ./csv2json.exe
REVISION      := `git rev-parse --short HEAD`
FLAG          :=  -a -tags netgo -trimpath -ldflags='-s -w -extldflags="-static" -buildid='

all:
	cat ./makefile
build:
	@make clean
	@export PATH=$$PWD:$$PATH; go build -o $(BIN)
release:
	@make clean
	@export PATH=$$PWD:$$PATH; go build $(FLAG) -o $(BIN)
	@make upx 
	@echo Success!
fmt:
	goimports -w *.go
	gofmt -w *.go
upx:
	upx --lzma $(BIN)
test:
	make build
	echo -e "a,b,c\n01,02,03\n11,12,13" | ./csv2json.exe

clean:
	rm -rf *.csv embedded_files.go
	go generate
	goimports -w *.go
	gofmt -w *.go
