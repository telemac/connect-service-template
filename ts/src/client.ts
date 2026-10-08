import { createClient, type Client } from "@connectrpc/connect";
import { createConnectTransport } from "@connectrpc/connect-web";
import { CustomerService } from "../gen/customer/v1/customer_pb.js";

export function newCustomerClient(baseUrl: string): Client<typeof CustomerService> {
  return createClient(CustomerService, createConnectTransport({ baseUrl, useHttpGet: true }));
}
