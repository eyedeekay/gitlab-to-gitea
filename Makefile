
fmt:
	@echo "Running gofumpt..."
	find . -name '*.go' -exec gofumpt -w -s -extra {} \;