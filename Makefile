all:
	@echo "Build targets:"
	@echo " docker"
	@echo " podman"

docker:
	docker build -t latex-report-server:latest .

podman:
	podman build -t latex-report-server:latest .
