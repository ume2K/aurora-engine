import http from "k6/http";
import { check, sleep } from "k6";

const BASE_URL = __ENV.BASE_URL || "http://localhost";

export const options = {
  vus: 50,
  duration: "30s",
  thresholds: {
    http_req_duration: ["p(95)<500"],
    http_req_failed: ["rate<0.01"],
  },
};

export function setup() {
  const email = `loadtest-${Date.now()}@test.local`;
  const password = "testpass123";

  const regRes = http.post(
    `${BASE_URL}/api/auth/register`,
    JSON.stringify({ email, password }),
    { headers: { "Content-Type": "application/json" } }
  );
  check(regRes, { "register ok": (r) => r.status === 200 || r.status === 201 });

  const loginRes = http.post(
    `${BASE_URL}/api/auth/login`,
    JSON.stringify({ email, password }),
    { headers: { "Content-Type": "application/json" } }
  );
  check(loginRes, { "login ok": (r) => r.status === 200 });

  const body = JSON.parse(loginRes.body);
  if (!body.token) {
    throw new Error("No token in login response");
  }

  return { token: body.token };
}

export default function (data) {
  const res = http.get(`${BASE_URL}/api/videos?limit=20`, {
    headers: { Authorization: `Bearer ${data.token}` },
  });

  check(res, {
    "status 200": (r) => r.status === 200,
    "is json": (r) => r.headers["Content-Type"]?.includes("application/json"),
  });

  sleep(0.1);
}
