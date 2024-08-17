import grpcweb from "k6/x/grpc-web";
import { check } from "k6";

const GRPC_WEB_ADDR = __ENV.GRPC_WEB_ADDR || "http://localhost:8080";

let client = new grpcweb.Client();

export default () => {
  if (__ITER === 0) {
    client.connect(GRPC_WEB_ADDR, {
      reflect: true,
    });
  }

  const response = client.invoke("/helloworld.Greeter/SayHello", {
    name: "name",
  });

  check(response, {
    "status is OK": (r) => r && r.status === grpcweb.StatusOK,
  });
  console.log(JSON.stringify(response));

  client.close();
};
