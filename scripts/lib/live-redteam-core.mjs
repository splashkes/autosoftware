import crypto from 'node:crypto';
import fs from 'node:fs/promises';
import path from 'node:path';

const MANIFEST_PATH = path.join('plans', 'live-anonymous-red-team', 'scenarios.json');

export async function loadLiveRedTeamProgram(repoRoot) {
  const raw = await fs.readFile(path.join(repoRoot, MANIFEST_PATH), 'utf8');
  const program = JSON.parse(raw);
  if (!program || !Array.isArray(program.scenarios)) {
    throw new Error('live red-team program is missing a scenarios array');
  }
  return program;
}

export function defaultRunID(now = new Date()) {
  return now.toISOString().replace(/[-:]/g, '').replace(/\.\d+Z$/, 'Z');
}

export function defaultOutputDir(repoRoot, runID, toolName) {
  return path.join(repoRoot, 'plans', 'live-anonymous-red-team', 'reports', 'runs', runID, toolName);
}

export function withinRunWindow(windowConfig, now = new Date()) {
  if (!windowConfig || !windowConfig.timezone) {
    return true;
  }
  const formatter = new Intl.DateTimeFormat('en-US', {
    hour: 'numeric',
    hourCycle: 'h23',
    timeZone: windowConfig.timezone
  });
  const parts = formatter.formatToParts(now);
  const hourPart = parts.find((part) => part.type === 'hour');
  const hour = hourPart ? Number(hourPart.value) : NaN;
  if (!Number.isFinite(hour)) {
    return false;
  }
  const startHour = Number(windowConfig.startHour);
  const endHour = Number(windowConfig.endHour);
  return hour >= startHour && hour < endHour;
}

export function selectScenarios(program, filters = {}) {
  const waveFilter = normalizeWaveFilter(filters.wave);
  return program.scenarios.filter((scenario) => {
    if (filters.driver && scenario.driver !== filters.driver) {
      return false;
    }
    if (waveFilter !== null && Number(scenario.wave) !== waveFilter) {
      return false;
    }
    if (filters.scenarioID && scenario.id !== filters.scenarioID) {
      return false;
    }
    if (filters.host && scenario.host !== filters.host) {
      return false;
    }
    return true;
  });
}

export function normalizeWaveFilter(value) {
  if (value === undefined || value === null || value === '' || value === 'all') {
    return null;
  }
  const parsed = Number(value);
  if (!Number.isInteger(parsed) || parsed < 1) {
    throw new Error(`invalid wave value: ${value}`);
  }
  return parsed;
}

export function normalizedHeaders(entries = {}) {
  const out = {};
  for (const [key, value] of Object.entries(entries)) {
    out[String(key).toLowerCase()] = String(value);
  }
  return out;
}

export function toPlainHeaders(headers) {
  const out = {};
  for (const [key, value] of headers.entries()) {
    out[String(key).toLowerCase()] = value;
  }
  return out;
}

export function sha256Hex(content) {
  return crypto.createHash('sha256').update(content).digest('hex');
}

export function clampScenarioRequestCount(program, scenario) {
  const requested = Number(scenario.requestTemplate?.requestCount || 1);
  const maxRequests = Number(scenario.maxRequests || requested);
  const classLimit = scenario.class === 'surface-mapping'
    ? Number(program.safetyGuards.discoveryMaxRequestsPerHost)
    : Number(program.safetyGuards.adversarialMaxRequestsPerScenario);
  return Math.max(1, Math.min(requested, maxRequests, classLimit));
}

export function clampScenarioConcurrency(program, scenario) {
  const requested = Number(scenario.maxConcurrency || 1);
  return Math.max(1, Math.min(requested, Number(program.safetyGuards.maxConcurrency || 10)));
}

export function p95(values) {
  if (!Array.isArray(values) || values.length === 0) {
    return 0;
  }
  const sorted = [...values].sort((a, b) => a - b);
  const index = Math.min(sorted.length - 1, Math.ceil(sorted.length * 0.95) - 1);
  return sorted[index];
}

export function ownerForScenario(program, scenario) {
  if (!scenario.ownerKey) {
    return null;
  }
  return program.owners?.[scenario.ownerKey] || null;
}

export function sleep(ms) {
  return new Promise((resolve) => setTimeout(resolve, ms));
}

export function booleanFromEnv(value) {
  switch (String(value || '').trim().toLowerCase()) {
  case '1':
  case 'true':
  case 'yes':
  case 'on':
    return true;
  default:
    return false;
  }
}
