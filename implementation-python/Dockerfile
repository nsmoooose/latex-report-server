# FROM texlive/texlive:latest
# FROM texlive/texlive:latest-medium
FROM texlive/texlive:latest-basic

RUN apt-get update && apt-get install -y \
    python3 python3-pip python3-flask

WORKDIR /app
COPY server.py /app

CMD ["python3", "server.py"]
