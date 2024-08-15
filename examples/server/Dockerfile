# syntax=docker/dockerfile:1

ARG GO_VERSION=1.22
FROM golang:${GO_VERSION} AS build
WORKDIR /src

RUN --mount=type=bind,source=go.sum,target=go.sum \
    --mount=type=bind,source=go.mod,target=go.mod \
    go mod download -x

RUN --mount=type=bind,target=. go build -o /bin/server

FROM debian:bookworm AS runtime

COPY --from=build /bin/server /bin/

EXPOSE 50051

ENTRYPOINT [ "/bin/server" ]
