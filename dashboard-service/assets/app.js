/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: MPL-2.0
 */

// Determine WebSocket URL based on current page location
var wsProtocol = window.location.protocol === "https:" ? "wss:" : "ws:";
var wsUrl = wsProtocol + "//" + window.location.host + "/ws";
var ws;
var reconnectTimer;

function connect() {
  ws = new WebSocket(wsUrl);

  ws.onopen = function () {
    console.log("WebSocket connected");
    didConnect();
    // Fetch count once on connect (refresh to update)
    requestCount();
  };

  ws.onmessage = function (event) {
    var message = JSON.parse(event.data);

    if (message.count < 0) {
      disconnectedFromBackendService();
    } else {
      didConnect();
    }

    showCount(message);
  };

  ws.onclose = function (event) {
    console.log("WebSocket closed:", event.code, event.reason);
    disconnected();
    // Reconnect after 2 seconds
    reconnectTimer = setTimeout(connect, 2000);
  };

  ws.onerror = function (error) {
    console.log("WebSocket error:", error);
    disconnected();
  };
}

function requestCount() {
  if (ws && ws.readyState === WebSocket.OPEN) {
    ws.send(JSON.stringify({ message: "get count" }));
  }
}

function showCount(message) {
  var formattedCount = Number(message.count).toLocaleString();
  var countEl = document.getElementById("count");
  var bannerEl = document.getElementById("error-banner");

  // Handle Error Message
  if (message.message) {
    bannerEl.textContent = message.message;
    bannerEl.style.display = "block";

    // Optional: make the count look like an error or placeholder
    if (message.count === -1) {
      countEl.textContent = "!";
      countEl.style.color = "var(--status-disconnected)";
    } else {
      countEl.textContent = formattedCount;
      countEl.style.color = ""; // reset
    }
  } else {
    bannerEl.style.display = "none";
    countEl.textContent = formattedCount;
    countEl.style.color = ""; // reset
  }

  document.getElementById("hostname").textContent = message.hostname;
  document.getElementById("dashboard-hostname").textContent =
    message.dashboard_hostname;

  var redisHostEl = document.getElementById("redis-hostname");
  if (redisHostEl) {
    redisHostEl.textContent = message.redis_host || "Unknown";
  }
}

function disconnected() {
  var el = document.getElementById("connection-status");
  el.classList.remove("connected");
  el.textContent = "Disconnected";
}

function disconnectedFromBackendService() {
  var el = document.getElementById("connection-status");
  el.classList.remove("connected");
  el.textContent = "Counting Service is Unreachable";
}

function didConnect() {
  var el = document.getElementById("connection-status");
  el.classList.add("connected");
  el.textContent = "Connected";
}

// Start connection
connect();
