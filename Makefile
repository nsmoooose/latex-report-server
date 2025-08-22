all:
	@echo "Build targets:"
	@echo " docker"
	@echo " podman"

docker:
	docker build -t latex-report-server:latest .

docker_run:
	docker run -it --rm --name reportserver -p 5000:5000 latex-report-server:latest

podman:
	podman build -t latex-report-server:latest .

podman_run:
	podman run -it --rm --name reportserver -p 5000:5000 latex-report-server:latest

