FROM texlive/texlive:latest

RUN apt-get update && apt-get install -y \
    python3 python3-pip python3-flask

WORKDIR /app
COPY server.py /app

CMD ["python3", "server.py"]
