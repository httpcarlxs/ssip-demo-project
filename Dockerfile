# syntax=docker/dockerfile:1
ARG BUILDKIT_SBOM_SCAN_CONTEXT=true
ARG BUILDKIT_SBOM_SCAN_STAGE=true

FROM cgr.dev/chainguard/go:latest as build

WORKDIR /app

COPY go.mod go.sum ./

RUN go mod download

COPY main.go ./

RUN CGO_ENABLED=0 GOOS=linux go build -o main

FROM cgr.dev/chainguard/static:latest

COPY --from=build /app/main /main

RUN addgroup -S ssipgroup && adduser -S ssip -G ssipgroup
USER ssip

EXPOSE 8080

CMD ["/main"]
