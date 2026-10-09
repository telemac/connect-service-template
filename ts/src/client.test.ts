// End-to-end tests of the generated TypeScript client against a running
// server. cmd/server/main_test.go starts one and sets BASE_URL; to run by hand:
// `make run`, then `BASE_URL=http://localhost:8080 npm test` in ts/.
import assert from "node:assert/strict";
import { test } from "node:test";
import { create } from "@bufbuild/protobuf";
import { createValidator } from "@bufbuild/protovalidate";
import { Code, ConnectError } from "@connectrpc/connect";
import { CreateCustomerRequestSchema } from "../gen/customer/v1/customer_pb.js";
import { newCustomerClient } from "./client.js";

const baseUrl = process.env.BASE_URL;
if (!baseUrl) {
  throw new Error("BASE_URL must point at a running server");
}
const client = newCustomerClient(baseUrl);

test("creates, reads and partially updates a customer", async () => {
  const { customer } = await client.createCustomer({
    customer: { name: "Ada", email: "ada@example.com" },
  });
  assert.ok(customer);

  // GetCustomer is NO_SIDE_EFFECTS, so the client sends it as an HTTP GET.
  const got = await client.getCustomer({ id: customer.id });
  assert.equal(got.customer?.name, "Ada");

  const updated = await client.updateCustomer({
    id: customer.id,
    customer: { email: "ada@lovelace.org" },
    updateMask: { paths: ["email"] },
  });
  assert.equal(updated.customer?.name, "Ada");
  assert.equal(updated.customer?.email, "ada@lovelace.org");
});

test("lists with filters and follows page tokens", async () => {
  // A unique marker keeps this test independent of other customers.
  const marker = `list-${crypto.randomUUID()}`;
  for (const n of [1, 2, 3]) {
    await client.createCustomer({ customer: { name: `${marker}-${n}`, email: `${n}@example.com` } });
  }

  const names: string[] = [];
  let pageToken = "";
  do {
    const page = await client.listCustomers({ nameContains: marker, pageSize: 2, pageToken });
    names.push(...page.customers.map((c) => c.name));
    pageToken = page.nextPageToken;
  } while (pageToken !== "");

  assert.deepEqual(names, [`${marker}-1`, `${marker}-2`, `${marker}-3`]);
});

test("server errors arrive as ConnectError with a code", async () => {
  await assert.rejects(
    client.createCustomer({ customer: { name: "Ada", email: "not-an-email" } }),
    (err: unknown) => err instanceof ConnectError && err.code === Code.InvalidArgument,
  );
  await assert.rejects(
    client.getCustomer({ id: crypto.randomUUID() }),
    (err: unknown) => err instanceof ConnectError && err.code === Code.NotFound,
  );
});

test("contract rules also run client-side, before any call", () => {
  const validator = createValidator();
  // Validation needs a real message instance, not a plain object.
  const result = validator.validate(
    CreateCustomerRequestSchema,
    create(CreateCustomerRequestSchema, { customer: { email: "ada@example.com" } }),
  );
  assert.equal(result.kind, "invalid");
  assert.match(result.error?.message ?? "", /customer\.name is required/);
});
