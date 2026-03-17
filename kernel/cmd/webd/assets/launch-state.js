(() => {
  function isTerminalExecutionStatus(statusValue) {
    return statusValue === "failed" || statusValue === "stopped" || statusValue === "terminated";
  }

  function latestExecutionEvent(events) {
    return Array.isArray(events) && events.length ? events[0] : null;
  }

  function hasExecutionEvent(events, name) {
    if (!Array.isArray(events)) return false;
    for (let index = 0; index < events.length; index += 1) {
      if (events[index] && events[index].name === name) return true;
    }
    return false;
  }

  function formatElapsed(ms) {
    const totalMs = Math.max(0, ms || 0);
    if (totalMs < 60000) {
      return (totalMs / 1000).toFixed(1) + "s";
    }
    const totalSeconds = Math.round(totalMs / 1000);
    const minutes = Math.floor(totalSeconds / 60);
    const seconds = totalSeconds % 60;
    return minutes + "m" + seconds + "s";
  }

  function stepLabel(session, events) {
    const statusValue = session && session.status ? session.status : "";
    if (statusValue === "launch_requested") return "Queued in kernel runtime";
    if (statusValue === "healthy") return session && session.open_path ? "Route ready" : "Registering route";
    if (statusValue === "failed") return "Launch failed";
    if (statusValue === "stopped") return "Launch stopped";
    if (statusValue === "terminated") return "Process terminated";
    if (hasExecutionEvent(events, "route_registered")) return "Route registered";
    if (hasExecutionEvent(events, "health_check_started")) return "Waiting for health check";
    if (hasExecutionEvent(events, "process_started")) return "Process started";
    if (hasExecutionEvent(events, "launch_spec_resolved")) return "Runtime manifest resolved";
    if (hasExecutionEvent(events, "launch_started")) return "Worker claimed launch";
    return "Starting execution";
  }

  function copyText(session, events, label, elapsedMs, transientError) {
    if (transientError) {
      return "Status polling was interrupted. Retrying against the runtime projection without abandoning the launch.";
    }
    const statusValue = session && session.status ? session.status : "";
    if (statusValue === "launch_requested") {
      if (elapsedMs > 15000) {
        return "Still waiting for the execution worker to claim the launch job and begin process startup.";
      }
      return "Writing the launch job into the runtime queue and waiting for the worker to claim it.";
    }
    if (statusValue === "healthy") {
      if (session && session.open_path) {
        return "The realization is healthy and routed. Opening " + session.open_path + " as soon as the launch trace settles.";
      }
      return "The realization is healthy. Waiting for the canonical route to finish registering.";
    }
    if (isTerminalExecutionStatus(statusValue)) {
      return (session && session.last_error) || ((label || "This realization") + " did not become runnable.");
    }
    if (hasExecutionEvent(events, "health_check_started")) {
      return "The process is running. Waiting for it to answer health checks on /healthz or its root route.";
    }
    if (hasExecutionEvent(events, "process_started")) {
      return "The process has started. Beginning health checks and route registration.";
    }
    if (hasExecutionEvent(events, "launch_spec_resolved")) {
      return "Runtime manifest resolved. Starting the process with kernel-provided capability URLs.";
    }
    if (hasExecutionEvent(events, "launch_started")) {
      return "The execution worker claimed the launch and is preparing the runtime.";
    }
    return "Preparing the runtime process, route bindings, and health checks.";
  }

  function progressPercent(session, events, elapsedMs) {
    const statusValue = session && session.status ? session.status : "";
    if (statusValue === "healthy") {
      return session && session.open_path ? 100 : 94;
    }
    if (isTerminalExecutionStatus(statusValue)) {
      return 100;
    }

    let progress = 8;
    if (statusValue === "launch_requested" || hasExecutionEvent(events, "launch_requested")) progress = 12;
    if (hasExecutionEvent(events, "launch_started")) progress = 24;
    if (hasExecutionEvent(events, "launch_spec_resolved")) progress = 42;
    if (hasExecutionEvent(events, "process_started") || (session && session.upstream_addr)) progress = 62;
    if (hasExecutionEvent(events, "health_check_started")) progress = 78 + Math.min(14, Math.floor(elapsedMs / 1500));
    if (hasExecutionEvent(events, "health_passed")) progress = 90;
    if (hasExecutionEvent(events, "route_registered") || (session && session.open_path)) progress = 96;
    return Math.max(8, Math.min(progress, 97));
  }

  function minimumProgressCap(launchContext, elapsedMs) {
    const minimumDisplayMs = Math.max(0, Number(launchContext && launchContext.minimumDisplayMs) || 0);
    if (minimumDisplayMs <= 0 || elapsedMs >= minimumDisplayMs) {
      return 100;
    }
    return 8 + ((96 - 8) * (elapsedMs / minimumDisplayMs));
  }

  function displayedProgress(session, events, launchContext, elapsedMs, allowCompletion) {
    const targetProgress = progressPercent(session, events, elapsedMs);
    if (allowCompletion) {
      return 100;
    }
    if (isTerminalExecutionStatus(session && session.status ? session.status : "")) {
      return 100;
    }
    return Math.max(8, Math.min(targetProgress, Math.round(minimumProgressCap(launchContext || {}, elapsedMs))));
  }

  function debugLine(session, events, launchContext, elapsedMs, transientError) {
    const latestEvent = latestExecutionEvent(events);
    const parts = [];
    parts.push("step=" + stepLabel(session, events));
    parts.push("status=" + ((session && session.status) || "unknown"));
    if (launchContext && launchContext.jobID) parts.push("job=" + launchContext.jobID);
    if (session && session.execution_id) parts.push("exec=" + session.execution_id);
    if (latestEvent && latestEvent.name) parts.push("event=" + latestEvent.name);
    if (session && session.upstream_addr) parts.push("upstream=" + session.upstream_addr);
    if (session && session.route_path_prefix) parts.push("route=" + session.route_path_prefix);
    if (session && session.open_path) parts.push("open=" + session.open_path);
    parts.push("elapsed=" + formatElapsed(elapsedMs));
    if (transientError) parts.push("transport=" + transientError);
    return parts.join(" | ");
  }

  window.ASLaunchState = {
    copyText,
    debugLine,
    displayedProgress,
    formatElapsed,
    hasExecutionEvent,
    isTerminalExecutionStatus,
    latestExecutionEvent,
    progressPercent,
    stepLabel,
  };
})();
