# Example scripts

## Setup

Build custom k6 binary with gRPC Web support.

```bash
$ xk6 build --with github.com/shota3506/xk6-grpc-web
```

Run envoy proxy and gRPC server by docker compose.

```bash
$ docker compose -f ./server/compose.yaml up
```

## Run test

### Unary RPC

```
$ ./k6 run ./unary.js -d 10s
```

### Server streaming RPC

```
$ ./k6 run ./server_streaming.js -d 10s
```
