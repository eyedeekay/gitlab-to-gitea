
fmt:
	@echo "Running gofumpt..."
	find . -name '*.go' -exec gofumpt -w -s -extra {} \;

CGO_ENABLED=0

all:	bin migrate unmigrate forkfix orgfix

bin:
	mkdir -p ./bin

migrate: bin
	go build --tags=netgo,osusergo -o ./bin/migrate ./cmd/migrate

unmigrate: bin
	go build --tags=netgo,osusergo -o ./bin/unmigrate ./cmd/unmigrate

forkfix: bin
	go build --tags=netgo,osusergo -o ./bin/forkfix ./cmd/forkfix

orgfix: bin
	go build --tags=netgo,osusergo -o ./bin/orgfix ./cmd/orgfix

mirror: bin
	go build --tags=netgo,osusergo -o ./bin/mirror ./cmd/mirror

namefix: bin
	go build --tags=netgo,osusergo -o ./bin/namefix ./cmd/namefix

clean:
	rm -f ./bin/migrate ./bin/unmigrate ./bin/forkfix ./bin/orgfix