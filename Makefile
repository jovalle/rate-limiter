.PHONY: fresh clean test test-integration test-unit lint docker-build helm-lint loadtest

generate:
	go generate

build: 
	go build

compile: generate build

build-image: compile
	docker build -t rate-limiter .

run-container: build-image
	sudo docker run --privileged --network host rate-limiter

run: generate build
	sudo ./rate-limiter

clean:
	rm -f *.o $(EXEC)
	rm -f rate-limiter
	rm -f rate_limiter_bpfel.o rate_limiter_bpfeb.o
	rm -f *.test
	rm -f coverage.out

fresh: clean

testapp:
	go run web/main.go

# Testing targets
test: test-unit

test-unit:
	go test -v -race -coverprofile=coverage.out ./...

test-integration:
	go test -v -run Integration ./...

test-short:
	go test -short -v ./...

# Linting
lint:
	golangci-lint run --timeout=5m

# Docker targets
docker-build:
	docker build -t rate-limiter:latest .

docker-run: docker-build
	docker run -d --name rate-limiter --privileged --network host \
		-e INTERFACE=eth0 -e LOG_LEVEL=info rate-limiter:latest

docker-stop:
	docker stop rate-limiter || true
	docker rm rate-limiter || true

# Kubernetes targets
k8s-deploy:
	kubectl apply -f k8s/

k8s-delete:
	kubectl delete -f k8s/

# Helm targets
helm-lint:
	helm lint helm/rate-limiter

helm-template:
	helm template test-release helm/rate-limiter

helm-install:
	helm install rate-limiter ./helm/rate-limiter \
		--namespace rate-limiter --create-namespace

helm-uninstall:
	helm uninstall rate-limiter --namespace rate-limiter

# Load testing
loadtest:
	cd loadtest && ./run-test.sh

loadtest-build:
	cd loadtest && go build -o loadtest main.go

# CI/CD helpers
ci-test: test lint

ci-build: compile

# Development helpers
dev-setup:
	go mod download
	go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest

# mon:
# 	sudo prometheus --config.file=monitoring/prometheus.yml
