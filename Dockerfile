# syntax=docker/dockerfile:1

# ---------- build ----------
FROM golang:1.26 AS build

WORKDIR /src

# No dependency-caching layer: the module is stdlib-only, so there is nothing
# for `go mod download` to fetch.
COPY . .

# The tests gate the binary — a failing table fails the image. This needs
# sample_clubes.jsonl in the build context, which pipeline_test.go reads as its
# golden fixture; see .dockerignore.
RUN go test ./...

# CGO_ENABLED=0 is not optional: distroless/static carries no libc, so the
# binary has to be statically linked. -s -w drop the symbol and DWARF tables.
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags='-s -w' -o /pipeline .

# ---------- run ----------
FROM gcr.io/distroless/static-debian12:nonroot

COPY --from=build /pipeline /pipeline

# clubs.csv and players.csv are written to the working directory, so /data is
# where you mount your own. Running as uid 65532, the container cannot write to
# a host directory you own unless you hand it your uid:
#
#   docker run --rm --user "$(id -u):$(id -g)" -v "$PWD:/data" \
#       clubs-pipeline sample_clubes.jsonl
WORKDIR /data

ENTRYPOINT ["/pipeline"]
