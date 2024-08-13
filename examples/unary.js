import grpcweb from "k6/x/grpc-web";
import { check } from "k6";

const GRPC_WEB_ADDR = __ENV.GRPC_WEB_ADDR || "http://localhost:8080";

let client = new grpcweb.Client();

client.load([], "./server/helloworld/helloworld.proto");

export default () => {
  client.connect(GRPC_WEB_ADDR);

  const response = client.invoke("/helloworld.Greeter/SayHello", {
    name: "name",
  });

  check(response, {
    "status is OK": (r) => r && r.status === grpcweb.StatusOK,
  });
  console.log(JSON.stringify(response));

  client.close();
};
