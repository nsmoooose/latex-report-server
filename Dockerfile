FROM golang:1.22-alpine AS build
WORKDIR /app
COPY main.go .
ENV CGO_ENABLED=0 GOOS=linux GOARCH=amd64
RUN go build -ldflags="-s -w" -o /out/latex-server main.go

# FROM texlive/texlive:latest
# FROM texlive/texlive:latest-medium
FROM texlive/texlive:latest-basic

COPY --from=build /out/latex-server /usr/bin/latex-server
CMD ["latex-server"]
