# xk6-grpc-web

[![Build](https://github.com/shota3506/xk6-grpc-web/actions/workflows/build.yaml/badge.svg)](https://github.com/shota3506/xk6-grpc-web/actions/workflows/build.yaml)
[![Go Report Card](https://goreportcard.com/badge/github.com/shota3506/xk6-grpc-web)](https://goreportcard.com/report/github.com/shota3506/xk6-grpc-web)

xk6-grpc-web is a [k6](https://k6.io/) extension that supports [gRPC-Web](https://grpc.io/docs/platforms/web/) protocol.

## Build

This extension is built using [xk6](https://github.com/grafana/xk6), a tool for building custom k6 binary.

```shell
xk6 build --with github.com/shota3506/xk6-grpc-web
```

## Example

### Unary RPC

```javascript
import grpcweb from "k6/x/grpc-web";
import { check } from "k6";

const GRPC_WEB_ADDR = __ENV.GRPC_WEB_ADDR;

let client = new grpcweb.Client();

client.load([], "PATH_TO_PROTO_FILE");

export default () => {
  client.connect(GRPC_WEB_ADDR);

  data = {};
  const response = client.invoke("/helloworld.Greeter/SayHello", data);

  check(response, {
    "status is OK": (r) => r && r.status === grpcweb.StatusOK,
  });

  console.log(JSON.stringify(response));

  client.close();
};
```

### Server streaming RPC

```javascript
import grpcweb from "k6/x/grpc-web";
import { sleep } from "k6";

const GRPC_WEB_ADDR = __ENV.GRPC_WEB_ADDR;

let client = new grpcweb.Client();

client.load([], "PATH_TO_PROTO_FILE");

export default () => {
  client.connect(GRPC_WEB_ADDR);

  data = {};
  const stream = client.stream("/helloworld.Greeter/SayRepeatHello", data);

  stream.on("data", (data) => {
    console.log("Data: " + JSON.stringify(data));
  });

  stream.on("error", (e) => {
    console.log("Error: " + JSON.stringify(e));
  });

  stream.on("end", () => {
    console.log("Done");
    client.close();
  });

  sleep(0.1);
};
```

See [examples](./examples) for runnable examples.
