
fmt:
	@echo "Running gofumpt..."
	find . -name '*.go' -exec gofumpt -w -s -extra {} \;

migrate:
	go build -o migrate ./cmd/migrate

unmigrate:
	go build -o unmigrate ./cmd/unmigrate

forkfix:
	go build -o forkfix ./cmd/forkfix
