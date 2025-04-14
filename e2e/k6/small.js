import http from "k6/http";
import { check, sleep } from "k6";
import { Rate } from "k6/metrics";
import { randomString } from "https://jslib.k6.io/k6-utils/1.2.0/index.js";

const baseUrl = "http://localhost:8081";
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
    baseline: {
      executor: "constant-arrival-rate",
      rate: 10, // 10 requests per second
      timeUnit: "1s",
      duration: "5m",
      preAllocatedVUs: 5,
      maxVUs: 20,
    },
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
const OWNER_IDS = ["670f63da5fd9588"];

// Setup function to create endpoints and subscriptions
export function setup() {
  // Create 10 subscription for each event
  for (let i = 0; i < 10; i++) {
    // 10 targets per owner
    const subscription = {
      owner_id: OWNER_IDS[0],
      event_types: EVENT_TYPES, // Make this variable lateron
      target: {
        url: "https://httpbin.org/post",
        method: "POST",
      },
    };

    const res = http.post(
      `${baseUrl}/subscriptions`,
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
    owner_id: "670f63da5fd95887",
    event_type: "webhook.incall",
  };

  const res = http.post(`${baseUrl}/event`, JSON.stringify(event), {
    headers: {
      "Content-Type": "application/json",
      "X-Hookrelay-Trace-Id": "webhook123",
    },
  });

  // Check response
  check(res, {
    "event accepted": (r) => r.status === 200,
    "response time < 500ms": (r) => r.timings.duration < 500,
  });

  sleep(Math.random() * 1 + 1);
}
