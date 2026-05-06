import { test, expect } from '@playwright/test';
import fs from 'node:fs';
import fsp from 'node:fs/promises';
import path from 'node:path';
import crypto from 'node:crypto';

type LiveProgram = {
  defaultRunWindow: {
    timezone: string;
    startHour: number;
    endHour: number;
  };
  owners: Record<string, { subsystem: string; repoPaths: string[] }>;
  scenarios: LiveScenario[];
};

type LiveScenario = {
  id: string;
  wave: number;
  driver: string;
  host: string;
  class: string;
  ownerKey?: string;
  manualOnly?: boolean;
  requestTemplate: Record<string, unknown>;
};

const repoRoot = path.resolve(__dirname, '..');
const program = JSON.parse(
  fs.readFileSync(path.join(repoRoot, 'plans', 'live-anonymous-red-team', 'scenarios.json'), 'utf8')
) as LiveProgram;

const attackerOrigin = process.env.LIVE_REDTEAM_ATTACKER_ORIGIN || `http://127.0.0.1:${process.env.LIVE_REDTEAM_ATTACKER_PORT || '40123'}`;
const allowOffWindow = envEnabled(process.env.LIVE_REDTEAM_ALLOW_OFF_WINDOW);
const waveFilter = process.env.LIVE_REDTEAM_WAVE || 'all';
const scenarioFilter = process.env.LIVE_REDTEAM_SCENARIO || '';
const hostFilter = process.env.LIVE_REDTEAM_HOST || '';

const browserScenarios = program.scenarios.filter((scenario) => {
  if (scenario.driver !== 'browser') {
    return false;
  }
  if (waveFilter !== 'all' && Number(scenario.wave) !== Number(waveFilter)) {
    return false;
  }
  if (scenarioFilter && scenario.id !== scenarioFilter) {
    return false;
  }
  if (hostFilter && scenario.host !== hostFilter) {
    return false;
  }
  return true;
});

test.describe.configure({ mode: 'serial' });

if (browserScenarios.length === 0) {
  test('no browser scenarios selected', async () => {
    test.skip(true, 'No browser scenarios matched the supplied filters.');
  });
}

for (const scenario of browserScenarios) {
  test(`[wave ${scenario.wave}] ${scenario.id}`, async ({ browser }, testInfo) => {
    test.skip(!allowOffWindow && !withinWindow(program.defaultRunWindow), 'Outside the default live-testing window.');

    const owner = scenario.ownerKey ? program.owners[scenario.ownerKey] : undefined;
    const harPath = testInfo.outputPath(`${scenario.id}.har`);
    const context = await browser.newContext({
      recordHar: { path: harPath }
    });
    const page = await context.newPage();
    const consoleErrors: string[] = [];
    page.on('console', (message) => {
      if (message.type() === 'error') {
        consoleErrors.push(message.text());
      }
    });

    const evidence: Record<string, unknown> = {
      id: scenario.id,
      wave: scenario.wave,
      host: scenario.host,
      class: scenario.class,
      ownerKey: scenario.ownerKey || '',
      ownerSubsystem: owner?.subsystem || '',
      startedAt: new Date().toISOString(),
      harPath
    };

    try {
      const action = String(scenario.requestTemplate.browserAction || '');
      if (action === 'frame-deny') {
        evidence.result = await runFrameDeny(page, scenario);
      } else if (action === 'csrf-form-post') {
        evidence.result = await runCSRFFromAttacker(page, scenario);
      } else if (action === 'cross-origin-fetch') {
        evidence.result = await runCrossOriginFetch(page, scenario);
      } else if (action === 'mounted-scan') {
        evidence.result = await runMountedScan(page, scenario);
      } else {
        throw new Error(`unsupported browser action: ${action}`);
      }
      evidence.consoleErrors = consoleErrors;
      evidence.finishedAt = new Date().toISOString();
      evidence.pageContentHash = crypto.createHash('sha256').update(await page.content()).digest('hex');
      await page.screenshot({ path: testInfo.outputPath(`${scenario.id}.png`), fullPage: true });
      await fsp.writeFile(testInfo.outputPath(`${scenario.id}.json`), JSON.stringify(evidence, null, 2) + '\n', 'utf8');
    } finally {
      await context.close();
    }
  });
}

async function runFrameDeny(page: import('@playwright/test').Page, scenario: LiveScenario) {
  const targetURL = String(scenario.requestTemplate.targetUrl || '');
  await page.goto(attackerOrigin, { waitUntil: 'domcontentloaded' });
  const responsePromise = page.waitForResponse((response) => response.url() === targetURL).catch(() => null);
  await page.setContent(`
    <!doctype html>
    <html lang="en">
    <body>
      <iframe id="probe" src="${escapeHTMLAttribute(targetURL)}"></iframe>
    </body>
    </html>
  `);
  await page.waitForTimeout(1500);
  const response = await responsePromise;
  const frameURLs = page.frames().map((frame) => frame.url());
  const blocked = !frameURLs.includes(targetURL);
  expect(blocked).toBeTruthy();
  return {
    action: 'frame-deny',
    targetURL,
    responseStatus: response?.status() ?? null,
    frameURLs,
    blocked
  };
}

async function runCSRFFromAttacker(page: import('@playwright/test').Page, scenario: LiveScenario) {
  const targetURL = String(scenario.requestTemplate.targetUrl || '');
  const formFields = (scenario.requestTemplate.formFields || {}) as Record<string, string>;
  const inputs = Object.entries(formFields)
    .map(([name, value]) => `<input name="${escapeHTMLAttribute(name)}" value="${escapeHTMLAttribute(value)}">`)
    .join('');
  await page.goto(attackerOrigin, { waitUntil: 'domcontentloaded' });
  const responsePromise = page.waitForResponse((response) => {
    return response.url() === targetURL && response.request().method() === 'POST';
  });
  await page.setContent(`
    <!doctype html>
    <html lang="en">
    <body>
      <iframe name="sink" style="display:none"></iframe>
      <form id="probe" action="${escapeHTMLAttribute(targetURL)}" method="POST" target="sink">
        ${inputs}
      </form>
      <script>document.getElementById('probe').submit();</script>
    </body>
    </html>
  `);
  const response = await responsePromise;
  expect(response.status()).toBe(403);
  return {
    action: 'csrf-form-post',
    targetURL,
    responseStatus: response.status(),
    location: response.headers()['location'] || null
  };
}

async function runCrossOriginFetch(page: import('@playwright/test').Page, scenario: LiveScenario) {
  const targetURL = String(scenario.requestTemplate.targetUrl || '');
  await page.goto(attackerOrigin, { waitUntil: 'domcontentloaded' });
  const responsePromise = page.waitForResponse((response) => response.url() === targetURL, { timeout: 8000 }).catch(() => null);
  const fetchResult = await page.evaluate(async (url) => {
    try {
      const controller = new AbortController();
      const timeout = window.setTimeout(() => controller.abort(), 8000);
      const response = await fetch(url, {
        method: 'GET',
        credentials: 'include',
        signal: controller.signal
      });
      const text = await response.text();
      window.clearTimeout(timeout);
      return {
        ok: true,
        status: response.status,
        bodyLength: text.length
      };
    } catch (error) {
      return {
        ok: false,
        error: String(error)
      };
    }
  }, targetURL);
  const networkResponse = await responsePromise;
  expect(fetchResult.ok).toBeFalsy();
  return {
    action: 'cross-origin-fetch',
    targetURL,
    jsResult: fetchResult,
    networkStatus: networkResponse?.status() ?? null,
    acao: networkResponse?.headers()['access-control-allow-origin'] || null
  };
}

async function runMountedScan(page: import('@playwright/test').Page, scenario: LiveScenario) {
  const targetURL = String(scenario.requestTemplate.targetUrl || '');
  const mountPrefix = String(scenario.requestTemplate.mountPrefix || '/');
  const maxLinks = Number(scenario.requestTemplate.maxLinks || 6);
  const queue = [targetURL];
  const visited = new Set<string>();
  const reports: Record<string, unknown>[] = [];

  while (queue.length > 0 && visited.size < maxLinks) {
    const current = queue.shift();
    if (!current || visited.has(current)) {
      continue;
    }
    visited.add(current);
    await page.goto(current, { waitUntil: 'domcontentloaded' });
    const report = await page.evaluate(({ mountPrefix: innerMountPrefix }) => {
      const elements = Array.from(document.querySelectorAll('*'));
      const attributeInventory: string[] = [];
      for (const element of elements) {
        for (const attr of Array.from(element.attributes)) {
          if (attr.name.startsWith('hx-') || attr.name === 'sse-connect') {
            attributeInventory.push(`${attr.name}=${attr.value}`);
          }
        }
      }

      const scripts = Array.from(document.querySelectorAll('script[src]'))
        .map((node) => node.getAttribute('src') || '')
        .filter(Boolean);
      const links = Array.from(document.querySelectorAll('a[href], link[href], form[action], script[src]'))
        .map((node) => {
          const raw = node.getAttribute('href') || node.getAttribute('action') || node.getAttribute('src') || '';
          try {
            return new URL(raw, window.location.href).pathname;
          } catch {
            return raw;
          }
        })
        .filter(Boolean);
      const mountBase = innerMountPrefix.endsWith('/') ? innerMountPrefix.slice(0, -1) : innerMountPrefix;
      const doublePrefix = links.filter((value) => value.startsWith(mountBase + innerMountPrefix));
      const samePrefixLinks = links.filter((value) => value.startsWith(innerMountPrefix));
      const rootAPILinks = links.filter((value) => value.startsWith('/v1/'));
      const reservedPaths = links.filter((value) => value.startsWith('/__runs/') || value.startsWith('/__sprout-assets/'));

      return {
        url: window.location.href,
        title: document.title,
        scripts,
        samePrefixLinks,
        rootAPILinks,
        reservedPaths,
        doublePrefix,
        attributeInventory
      };
    }, { mountPrefix });
    reports.push(report);
    for (const href of (report.samePrefixLinks as string[]).slice(0, maxLinks)) {
      const absolute = new URL(href, current).toString();
      if (!visited.has(absolute) && queue.length < maxLinks) {
        queue.push(absolute);
      }
    }
  }

  const doublePrefixed = reports.flatMap((report) => (report.doublePrefix as string[]) || []);
  expect(doublePrefixed).toHaveLength(0);
  return {
    action: 'mounted-scan',
    targetURL,
    scannedPages: reports
  };
}

function withinWindow(windowConfig: LiveProgram['defaultRunWindow']) {
  const formatter = new Intl.DateTimeFormat('en-US', {
    hour: 'numeric',
    hourCycle: 'h23',
    timeZone: windowConfig.timezone
  });
  const parts = formatter.formatToParts(new Date());
  const hour = Number(parts.find((part) => part.type === 'hour')?.value || NaN);
  return Number.isFinite(hour) && hour >= windowConfig.startHour && hour < windowConfig.endHour;
}

function envEnabled(value: string | undefined) {
  switch ((value || '').trim().toLowerCase()) {
  case '1':
  case 'true':
  case 'yes':
  case 'on':
    return true;
  default:
    return false;
  }
}

function escapeHTMLAttribute(value: string) {
  return value
    .replace(/&/g, '&amp;')
    .replace(/"/g, '&quot;')
    .replace(/</g, '&lt;')
    .replace(/>/g, '&gt;');
}
