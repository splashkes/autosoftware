package feedbackloop

import (
	"encoding/json"
	"fmt"
)

const DefaultIncidentEndpointPath = "/__kernel/feedback-loop/incidents"

type ClientReporterConfig struct {
	EndpointPath         string `json:"endpointPath"`
	SeedID               string `json:"seedId,omitempty"`
	RealizationID        string `json:"realizationId,omitempty"`
	RequestID            string `json:"requestId,omitempty"`
	SessionID            string `json:"sessionId,omitempty"`
	IncludeConsoleErrors bool   `json:"includeConsoleErrors"`
	IncludeHTMX          bool   `json:"includeHTMX"`
}

func (cfg ClientReporterConfig) Script() string {
	if cfg.EndpointPath == "" {
		cfg.EndpointPath = DefaultIncidentEndpointPath
	}

	payload, err := json.Marshal(cfg)
	if err != nil {
		panic(err)
	}

	return fmt.Sprintf(`(function () {
  var cfg = %s;
  if (!cfg.endpointPath) {
    return;
  }

  function stringify(value) {
    if (value instanceof Error) {
      return value.message || value.toString();
    }
    if (typeof value === "string") {
      return value;
    }
    try {
      return JSON.stringify(value);
    } catch (err) {
      return String(value);
    }
  }

  function send(kind, severity, payload) {
    try {
      var body = JSON.stringify({
        kind: kind,
        severity: severity,
        message: payload.message || kind,
        stack: payload.stack || "",
        component_stack: payload.componentStack || "",
        source: payload.source || "",
        created_at: new Date().toISOString(),
        request: {
          request_id: cfg.requestId || "",
          session_id: cfg.sessionId || "",
          seed_id: cfg.seedId || "",
          realization_id: cfg.realizationId || "",
          route: window.location.pathname || "",
          method: "BROWSER",
          page_url: window.location.href || "",
          referrer: document.referrer || "",
          user_agent: navigator.userAgent || ""
        },
        tags: payload.tags || {},
        data: payload.data || {}
      });

      if (navigator.sendBeacon) {
        navigator.sendBeacon(
          cfg.endpointPath,
          new Blob([body], { type: "application/json" })
        );
        return;
      }

      fetch(cfg.endpointPath, {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        credentials: "same-origin",
        keepalive: true,
        body: body
      }).catch(function () {});
    } catch (err) {
    }
  }

  window.addEventListener("error", function (event) {
    send("window.error", "error", {
      message: event.message || "Unhandled error",
      stack: event.error && event.error.stack || "",
      source: event.filename || "",
      data: {
        lineno: event.lineno || 0,
        colno: event.colno || 0
      }
    });
  });

  window.addEventListener("unhandledrejection", function (event) {
    var reason = event.reason;
    send("window.unhandledrejection", "error", {
      message: stringify(reason),
      stack: reason && reason.stack || "",
      data: {
        type: typeof reason
      }
    });
  });

  if (cfg.includeConsoleErrors && window.console && typeof window.console.error === "function") {
    var originalConsoleError = window.console.error;
    window.console.error = function () {
      var message = Array.prototype.map.call(arguments, stringify).join(" ");
      send("console.error", "error", {
        message: message
      });
      return originalConsoleError.apply(this, arguments);
    };
  }

  if (cfg.includeHTMX && document.body) {
    [
      "htmx:responseError",
      "htmx:sendError",
      "htmx:swapError",
      "htmx:targetError"
    ].forEach(function (eventName) {
      document.body.addEventListener(eventName, function (event) {
        var detail = event.detail || {};
        send(eventName, "error", {
          message: eventName,
          data: {
            request_path: detail.pathInfo && detail.pathInfo.requestPath || "",
            status: detail.xhr && detail.xhr.status || 0
          }
        });
      });
    });
  }
})();`, string(payload))
}
