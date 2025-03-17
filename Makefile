.PHONY: build clean test report aws gcp azure

# Default target
all: build

# Build the project
build:
	go get 
	go mod tidy
	go build -o observability-cost-center .

# Clean build artifacts
clean:
	rm -f observability-cost-center

# Run tests
test:
	go test ./...

# Provider targets that can be used directly
aws:
	./observability-cost-center --output-file report.txt --output summary --config observability-cost-center.yaml report --provider aws 
# Special target to handle "make report aws" syntax
report:
	@provider="$(filter-out $@,$(MAKECMDGOALS))"; \
	if [ -z "$$provider" ]; then \
		echo "Usage: make report aws|gcp|azure"; \
		exit 1; \
	fi; \
	$(MAKE) $$provider

# Empty rule to handle the second argument
%:
	@:
