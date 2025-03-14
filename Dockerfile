# syntax=docker/dockerfile:1
ARG BUILDKIT_SBOM_SCAN_CONTEXT=true
ARG BUILDKIT_SBOM_SCAN_STAGE=true

FROM cgr.dev/chainguard/go:latest AS build

WORKDIR /app

COPY go.mod go.sum ./

RUN go mod download

COPY main.go ./

RUN CGO_ENABLED=0 GOOS=linux go build -o main

FROM cgr.dev/chainguard/wolfi-base:latest

RUN addgroup -S ssipgroup && adduser -S ssip -G ssipgroup

COPY --from=build /app/main /main

USER ssip

EXPOSE 8080

CMD ["/main"]
