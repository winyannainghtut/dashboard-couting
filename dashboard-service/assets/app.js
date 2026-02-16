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
  document.getElementById("count").textContent = formattedCount;
  document.getElementById("hostname").textContent = message.hostname;
  document.getElementById("dashboard-hostname").textContent =
    message.dashboard_hostname;
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
