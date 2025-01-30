import http from "k6/http";
import { check, sleep } from "k6";
import { Rate } from "k6/metrics";
import { randomString } from "https://jslib.k6.io/k6-utils/1.2.0/index.js";

// Custom metrics
const errorRate = new Rate("errors");
const fanoutRate = new Rate("fanout_success");

// Test configuration
export const options = {
  scenarios: {
    // Setup scenario - runs once to create endpoints and subscriptions
    setup: {
      executor: "shared-iterations",
      vus: 1,
      iterations: 1,
      exec: "setup",
    },
    // Main load test scenario
    load: {
      executor: "ramping-vus",
      startVUs: 0,
      stages: [
        { duration: "2m", target: 100 },
        { duration: "5m", target: 100 },
        { duration: "2m", target: 200 },
        { duration: "5m", target: 200 },
        { duration: "2m", target: 0 },
      ],
      startTime: "10s", // Start after setup
      exec: "default",
    },
    // Webhook receiver scenario - simulates endpoints receiving events
    // receiver: {
    //   executor: "constant-vus",
    //   vus: 10, // Number of webhook receivers
    //   duration: "15m",
    //   exec: "webhookReceiver",
    //   startTime: "10s",
    // },
  },
  thresholds: {
    http_req_duration: ["p(95)<500"],
    errors: ["rate<0.1"],
    // fanout_success: ["rate>0.95"], // 95% of fan-outs should succeed
  },
};

// Shared state between VUs
let endpoints = [];
let subscriptions = [];
const EVENT_TYPES = [
  "order.created",
  "user.signup",
  "payment.success",
  "inventory.updated",
];
// const OWNER_IDS = ["owner1", "owner2", "owner3"];
const OWNER_IDS = ["owner"];

// Setup function to create endpoints and subscriptions
export function setup() {
  // Create 10 subscription for each event
  for (let i = 0; i < 10; i++) {
    // 10 targets per owner
    const subscription = {
      owner_id: `owner_${i}`,
      event_types: EVENT_TYPES, // Make this variable lateron
      target: {
        type: "http",
        http_details: {
          url: "https://httpbin.org/post",
          method: "POST",
        },
      },
    };

    const res = http.post(
      `${__ENV.BASE_URL}:8081/subscriptions`,
      JSON.stringify(subscription),
      {
        headers: { "Content-Type": "application/json" },
      },
    );
  }
}

// Main test function for event generation
export default function (data) {
  // Generate event matching existing subscriptions
  // const owner = OWNER_IDS[Math.floor(Math.random() * OWNER_IDS.length)];
  const owner = OWNER_IDS[0];
  const eventType = EVENT_TYPES[0];

  const event = {
    payload: {
      message: `Test message ${randomString(10)}`,
      timestamp: new Date().toISOString(),
      data: { foo: "bar" },
    },
    owner_id: owner,
    event_type: eventType,
  };

  const res = http.post(`${__ENV.BASE_URL}:8082/event`, JSON.stringify(event), {
    headers: { "Content-Type": "application/json" },
  });

  // Check response
  check(res, {
    "event accepted": (r) => r.status === 200,
    "response time < 500ms": (r) => r.timings.duration < 500,
  });

  sleep(Math.random() * 1 + 1);
}

// Webhook receiver simulation
// export function webhookReceiver() {
//   // This represents your mock endpoints receiving webhooks
//   // You'll need to actually implement these endpoints in your test environment
//   const endpoint = endpoints[Math.floor(Math.random() * endpoints.length)];
//
//   // Simulate processing time and success/failure
//   sleep(Math.random() * 0.5); // Simulate processing time 0-500ms
//
//   // Record success rate for fan-out delivery
//   fanoutRate.add(Math.random() < 0.95); // Simulate 95% success rate
// }
