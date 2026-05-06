#!/usr/bin/env node

import fs from 'node:fs/promises';
import path from 'node:path';
import { fileURLToPath } from 'node:url';

import {
  booleanFromEnv,
  clampScenarioConcurrency,
  clampScenarioRequestCount,
  defaultOutputDir,
  defaultRunID,
  loadLiveRedTeamProgram,
  normalizedHeaders,
  ownerForScenario,
  p95,
  selectScenarios,
  sha256Hex,
  sleep,
  toPlainHeaders,
  withinRunWindow
} from './lib/live-redteam-core.mjs';

const __dirname = path.dirname(fileURLToPath(import.meta.url));
const repoRoot = path.resolve(__dirname, '..');

const options = parseArgs(process.argv.slice(2));
const program = await loadLiveRedTeamProgram(repoRoot);

if (options.help) {
  printHelp();
  process.exit(0);
}

if (options.list) {
  printScenarioList(program, options);
  process.exit(0);
}

if (!options.allowOffWindow && !withinRunWindow(program.defaultRunWindow)) {
  throw new Error('outside the default live-testing window; pass --allow-off-window to override');
}

const runID = options.runID || process.env.LIVE_REDTEAM_RUN_ID || defaultRunID();
const outputDir = options.outputDir
  ? path.resolve(options.outputDir)
  : defaultOutputDir(repoRoot, runID, 'http');
await fs.mkdir(outputDir, { recursive: true });

const evidencePath = path.join(outputDir, 'evidence.jsonl');
const summaryPath = path.join(outputDir, 'summary.json');
const baselineOutputPath = path.join(outputDir, 'baseline.json');
const latestBaselinePath = path.join(repoRoot, 'plans', 'live-anonymous-red-team', 'reports', 'latest-baseline.json');

const selectedScenarios = selectScenarios(program, {
  driver: 'http',
  wave: options.wave,
  scenarioID: options.scenarioID,
  host: options.host
});

if (selectedScenarios.length === 0) {
  throw new Error('no HTTP scenarios matched the supplied filters');
}

const baseline = options.baselinePath ? await readJSON(options.baselinePath) : null;
const hostDiscoveryCounts = new Map();
const launchHistory = [];
const summary = {
  runID,
  startedAt: new Date().toISOString(),
  operatorNote: options.note || '',
  filters: {
    wave: options.wave || 'all',
    scenarioID: options.scenarioID || '',
    host: options.host || '',
    includeManual: options.includeManual,
    dryRun: options.dryRun
  },
  scenarios: [],
  baselinePath: options.baselinePath || '',
  outputDir
};

for (const scenario of selectedScenarios) {
  const scenarioSummary = await runScenario({
    program,
    scenario,
    options,
    outputDir,
    evidencePath,
    baseline,
    hostDiscoveryCounts,
    launchHistory
  });
  summary.scenarios.push(scenarioSummary);
}

summary.finishedAt = new Date().toISOString();
summary.waveBaseline = buildBaseline(summary.scenarios);

await fs.writeFile(summaryPath, JSON.stringify(summary, null, 2) + '\n', 'utf8');
if (Object.keys(summary.waveBaseline).length > 0) {
  await fs.writeFile(baselineOutputPath, JSON.stringify(summary.waveBaseline, null, 2) + '\n', 'utf8');
  await fs.writeFile(latestBaselinePath, JSON.stringify(summary.waveBaseline, null, 2) + '\n', 'utf8');
}

console.log(`live red-team HTTP run complete: ${summaryPath}`);

async function runScenario(context) {
  const { program, scenario, options, evidencePath, baseline, hostDiscoveryCounts, launchHistory } = context;
  const owner = ownerForScenario(program, scenario);
  const requestCount = clampScenarioRequestCount(program, scenario);
  const concurrency = clampScenarioConcurrency(program, scenario);
  const summary = {
    id: scenario.id,
    wave: scenario.wave,
    host: scenario.host,
    class: scenario.class,
    ownerKey: scenario.ownerKey || '',
    ownerSubsystem: owner?.subsystem || '',
    manualOnly: Boolean(scenario.manualOnly),
    requestCount,
    maxConcurrency: concurrency,
    status: 'planned',
    notes: [],
    records: [],
    observedStatuses: {},
    latenciesMs: []
  };

  if (scenario.manualOnly && !options.includeManual) {
    summary.status = 'skipped';
    summary.notes.push('manual-only scenario skipped');
    return summary;
  }

  if (scenario.class === 'surface-mapping') {
    const used = hostDiscoveryCounts.get(scenario.host) || 0;
    const cap = Number(program.safetyGuards.discoveryMaxRequestsPerHost);
    if (used + requestCount > cap) {
      summary.status = 'skipped';
      summary.notes.push(`discovery cap reached for ${scenario.host}`);
      return summary;
    }
    hostDiscoveryCounts.set(scenario.host, used + requestCount);
  }

  if (scenario.launchScenario) {
    const cooldownMinutes = Number(program.safetyGuards.successfulLaunchCooldownMinutes || 10);
    const lastLaunch = launchHistory[launchHistory.length - 1];
    if (lastLaunch && (Date.now() - lastLaunch) < cooldownMinutes * 60 * 1000) {
      summary.status = 'skipped';
      summary.notes.push(`launch cooldown active for ${cooldownMinutes} minutes`);
      return summary;
    }
  }

  if (scenario.healthGate) {
    const healthProbe = await performRequest(program, scenario, {
      url: scenario.healthGate.url,
      method: 'GET',
      headers: { accept: 'text/html', 'cache-control': 'no-cache' },
      requestIndex: 'health-gate'
    });
    summary.healthGate = {
      url: scenario.healthGate.url,
      status: healthProbe.status,
      latencyMs: healthProbe.latencyMs,
      xAsRequestID: healthProbe.xAsRequestID,
      cfRay: healthProbe.cfRay
    };
    if (!scenario.healthGate.healthyStatuses.includes(healthProbe.status)) {
      summary.status = 'skipped';
      summary.notes.push(`health gate failed with status ${healthProbe.status}`);
      return summary;
    }
  }

  if (options.dryRun) {
    summary.status = 'dry-run';
    summary.notes.push('scenario selected but not executed');
    return summary;
  }

  let consecutiveUnexpected5xx = 0;
  summary.status = 'completed';
  for (let requestIndex = 0; requestIndex < requestCount; requestIndex += concurrency) {
    const batch = [];
    for (let offset = 0; offset < concurrency && (requestIndex + offset) < requestCount; offset += 1) {
      batch.push(runAttempt(program, scenario, requestIndex + offset));
    }
    const results = await Promise.all(batch);
    for (const result of results) {
      summary.records.push(result);
      summary.latenciesMs.push(result.latencyMs);
      summary.observedStatuses[result.status] = (summary.observedStatuses[result.status] || 0) + 1;
      await appendJSONL(evidencePath, {
        runID: context.options.runID || process.env.LIVE_REDTEAM_RUN_ID || '',
        scenarioID: scenario.id,
        ownerKey: scenario.ownerKey || '',
        timestamp: result.timestamp,
        host: scenario.host,
        method: result.method,
        url: result.url,
        headersSent: result.headersSent,
        status: result.status,
        latencyMs: result.latencyMs,
        xAsRequestID: result.xAsRequestID,
        cfRay: result.cfRay,
        responseHash: result.responseHash,
        location: result.location
      });

      if (isUnexpected5xx(scenario, result.status)) {
        consecutiveUnexpected5xx += 1;
      } else {
        consecutiveUnexpected5xx = 0;
      }

      if (!matchesExpectedStatus(scenario, result.status)) {
        summary.notes.push(`unexpected status ${result.status} on ${result.url}`);
      }

      if (scenario.launchScenario && isLikelySuccessfulLaunch(result)) {
        launchHistory.push(Date.now());
        summary.notes.push('launch probe returned a likely successful launch response');
      }
    }

    if (consecutiveUnexpected5xx >= 2) {
      summary.status = 'aborted';
      summary.notes.push('aborted after 2 consecutive unexpected 5xx responses');
      summary.availabilityProbe = await probeBroadAvailabilityLoss(scenario.host);
      break;
    }

    const hostBaseline = baseline?.[scenario.host];
    if (hostBaseline && summary.latenciesMs.length >= 10 && p95(summary.latenciesMs) > (hostBaseline.p95Ms || 0) * 3) {
      summary.status = 'aborted';
      summary.notes.push(`aborted because p95 latency ${p95(summary.latenciesMs)}ms exceeded 3x baseline ${hostBaseline.p95Ms}ms`);
      break;
    }

    const intervalMs = Number(scenario.requestTemplate?.requestIntervalMs || 0);
    if (intervalMs > 0 && (requestIndex + concurrency) < requestCount) {
      await sleep(intervalMs);
    }
  }

  summary.p95Ms = p95(summary.latenciesMs);
  return summary;
}

async function runAttempt(program, scenario, requestIndex) {
  return performRequest(program, scenario, {
    url: scenario.requestTemplate.url,
    method: scenario.requestTemplate.method,
    headers: scenario.requestTemplate.headers,
    body: scenario.requestTemplate.body,
    jsonBody: scenario.requestTemplate.jsonBody,
    redirect: scenario.requestTemplate.redirect,
    requestIndex
  });
}

async function performRequest(program, scenario, request) {
  const started = Date.now();
  const headers = normalizedHeaders(request.headers || {});
  let body = request.body;
  if (request.jsonBody !== undefined) {
    body = JSON.stringify(request.jsonBody);
    if (!headers['content-type']) {
      headers['content-type'] = 'application/json';
    }
  }

  const controller = new AbortController();
  const timeoutMs = Number(scenario.requestTemplate?.timeoutMs || 10000);
  const timeout = setTimeout(() => controller.abort(), timeoutMs);

  try {
    const response = await fetch(request.url, {
      method: request.method,
      headers,
      body,
      redirect: request.redirect || 'follow',
      signal: controller.signal
    });
    const rawBody = Buffer.from(await response.arrayBuffer());
    const responseHash = sha256Hex(rawBody);
    return {
      timestamp: new Date().toISOString(),
      url: request.url,
      method: request.method,
      headersSent: headers,
      requestIndex: request.requestIndex,
      status: response.status,
      latencyMs: Date.now() - started,
      xAsRequestID: response.headers.get('x-as-request-id') || '',
      cfRay: response.headers.get('cf-ray') || '',
      responseHash,
      location: response.headers.get('location') || '',
      responseHeaders: toPlainHeaders(response.headers),
      contentType: response.headers.get('content-type') || ''
    };
  } catch (error) {
    return {
      timestamp: new Date().toISOString(),
      url: request.url,
      method: request.method,
      headersSent: headers,
      requestIndex: request.requestIndex,
      status: 0,
      latencyMs: Date.now() - started,
      xAsRequestID: '',
      cfRay: '',
      responseHash: sha256Hex(String(error)),
      location: '',
      error: String(error)
    };
  } finally {
    clearTimeout(timeout);
  }
}

async function probeBroadAvailabilityLoss(host) {
  const url = `https://${host}/`;
  const probe = await performRequest({}, { requestTemplate: { timeoutMs: 5000 } }, {
    url,
    method: 'GET',
    headers: {
      accept: 'text/html',
      'cache-control': 'no-cache'
    },
    requestIndex: 'availability-probe'
  });
  return {
    url,
    status: probe.status,
    latencyMs: probe.latencyMs,
    xAsRequestID: probe.xAsRequestID,
    cfRay: probe.cfRay
  };
}

function matchesExpectedStatus(scenario, status) {
  if (!Array.isArray(scenario.expectedStatuses) || scenario.expectedStatuses.length === 0) {
    return true;
  }
  return scenario.expectedStatuses.includes(status);
}

function isUnexpected5xx(scenario, status) {
  return status >= 500 && status <= 599 && !matchesExpectedStatus(scenario, status);
}

function isLikelySuccessfulLaunch(result) {
  if (result.status !== 200) {
    return false;
  }
  return true;
}

function buildBaseline(scenarios) {
  const samples = new Map();
  for (const scenario of scenarios) {
    if (scenario.class !== 'surface-mapping') {
      continue;
    }
    const bucket = samples.get(scenario.host) || [];
    for (const latency of scenario.latenciesMs || []) {
      bucket.push(latency);
    }
    samples.set(scenario.host, bucket);
  }

  const baseline = {};
  for (const [host, values] of samples.entries()) {
    baseline[host] = {
      count: values.length,
      p95Ms: p95(values)
    };
  }
  return baseline;
}

async function appendJSONL(filename, payload) {
  await fs.appendFile(filename, JSON.stringify(payload) + '\n', 'utf8');
}

async function readJSON(filename) {
  const raw = await fs.readFile(path.resolve(filename), 'utf8');
  return JSON.parse(raw);
}

function printScenarioList(program, options) {
  const scenarios = selectScenarios(program, {
    driver: 'http',
    wave: options.wave,
    scenarioID: options.scenarioID,
    host: options.host
  });
  for (const scenario of scenarios) {
    console.log(`${scenario.wave}\t${scenario.id}\t${scenario.host}\t${scenario.class}${scenario.manualOnly ? '\tmanual' : ''}`);
  }
}

function printHelp() {
  console.log(`Usage: node scripts/live-redteam-http.mjs [options]

Options:
  --list                  List HTTP scenarios
  --wave <n|all>          Filter by wave number
  --scenario <id>         Run one scenario
  --host <host>           Filter by host
  --output-dir <path>     Override the output directory
  --baseline <path>       Read a baseline JSON file from a previous Wave 1 run
  --include-manual        Include manual-only scenarios
  --allow-off-window      Bypass the 01:00-05:00 America/Toronto run window
  --dry-run               Select scenarios without executing them
  --note <text>           Operator note recorded in the summary
  --run-id <id>           Override the run identifier
  --help                  Show this help
`);
}

function parseArgs(argv) {
  const parsed = {
    allowOffWindow: booleanFromEnv(process.env.LIVE_REDTEAM_ALLOW_OFF_WINDOW),
    baselinePath: process.env.LIVE_REDTEAM_BASELINE || '',
    dryRun: false,
    help: false,
    host: process.env.LIVE_REDTEAM_HOST || '',
    includeManual: booleanFromEnv(process.env.LIVE_REDTEAM_INCLUDE_MANUAL),
    list: false,
    note: process.env.LIVE_REDTEAM_NOTE || '',
    outputDir: process.env.LIVE_REDTEAM_OUTPUT_DIR || '',
    runID: process.env.LIVE_REDTEAM_RUN_ID || '',
    scenarioID: process.env.LIVE_REDTEAM_SCENARIO || '',
    wave: process.env.LIVE_REDTEAM_WAVE || 'all'
  };

  for (let index = 0; index < argv.length; index += 1) {
    const arg = argv[index];
    switch (arg) {
    case '--allow-off-window':
      parsed.allowOffWindow = true;
      break;
    case '--baseline':
      parsed.baselinePath = argv[++index] || '';
      break;
    case '--dry-run':
      parsed.dryRun = true;
      break;
    case '--help':
      parsed.help = true;
      break;
    case '--host':
      parsed.host = argv[++index] || '';
      break;
    case '--include-manual':
      parsed.includeManual = true;
      break;
    case '--list':
      parsed.list = true;
      break;
    case '--note':
      parsed.note = argv[++index] || '';
      break;
    case '--output-dir':
      parsed.outputDir = argv[++index] || '';
      break;
    case '--run-id':
      parsed.runID = argv[++index] || '';
      break;
    case '--scenario':
      parsed.scenarioID = argv[++index] || '';
      break;
    case '--wave':
      parsed.wave = argv[++index] || 'all';
      break;
    default:
      throw new Error(`unknown argument: ${arg}`);
    }
  }

  return parsed;
}
