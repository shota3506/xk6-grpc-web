import grpcweb from "k6/x/grpc-web";
import { sleep } from "k6";

const GRPC_WEB_ADDR = __ENV.GRPC_WEB_ADDR || "http://localhost:8080";

let client = new grpcweb.Client();

client.load([], "./server/helloworld/helloworld.proto");

export default () => {
  client.connect(GRPC_WEB_ADDR);

  const stream = client.stream("/helloworld.Greeter/SayRepeatHello", {
    name: "name",
    count: 10,
  });

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
