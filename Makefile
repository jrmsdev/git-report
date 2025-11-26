.PHONY: all
all: build-deps build

.PHONY: fmt
fmt:
	@go fmt

.PHONY: clean
clean:
	@rm -vrf *.log *.db build main git-report

.PHONY: build-deps
build-deps:
	@go get github.com/mattn/go-sqlite3
	@go get gopkg.in/yaml.v3

.PHONY: build
build: build/git-report

build/git-report: main.go
	@mkdir -vp build
	@CGO_ENABLED=1 go build -o build/git-report main.go

.PHONY: install
install:
	@CGO_ENABLED=1 go install

.PHONY: run
run: build
	@build/git-report
