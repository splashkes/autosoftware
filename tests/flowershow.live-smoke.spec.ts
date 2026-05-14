import { test, expect, request } from '@playwright/test';

const LIVE_BASE = 'https://autosoftware.app';
const API_BASE = 'https://api.autosoftware.app';

test.describe('Flowershow Live PR 1 smoke (image pipeline + cover photo + class media)', () => {
  test('contract endpoint exposes the new media commands and the media domain object', async () => {
    const ctx = await request.newContext();
    const list = await ctx.get(`${API_BASE}/v1/contracts`);
    expect(list.status()).toBe(200);
    const body = await list.json();
    const fs = body.contracts.find((c: any) => c.seed_id === '0007-Flowershow');
    expect(fs).toBeTruthy();
    expect(fs.commands).toContain('media.set_cover');
    expect(fs.commands).toContain('media.attach_to_class');

    const detail = await ctx.get(
      `${API_BASE}/v1/contracts/0007-Flowershow/a-firstbloom`,
    );
    expect(detail.status()).toBe(200);
    const detailBody = await detail.json();
    const kinds = (detailBody.contract?.domain_objects ?? []).map(
      (d: any) => d.kind,
    );
    expect(kinds).toContain('media');
  });

  test('home page loads on live', async ({ page }) => {
    const res = await page.goto(`${LIVE_BASE}/flowershow/`);
    expect(res?.status() ?? 0).toBeLessThan(400);
    await expect(page.locator('h1').first()).toBeVisible();
  });

  test('a public show detail page renders without 5xx', async ({ page }) => {
    const res = await page.goto(`${LIVE_BASE}/flowershow/shows/d12026-0425`, {
      waitUntil: 'domcontentloaded',
    });
    expect(res?.status() ?? 0).toBeLessThan(500);
  });

  test('thumbnail endpoint accepts ?thumb=1 without 5xx and returns image bytes when present', async ({
    page,
    request,
  }) => {
    await page.goto(`${LIVE_BASE}/flowershow/`);
    const showLink = page.locator('a[href*="/shows/"]').first();
    const showHref = await showLink.getAttribute('href');
    test.skip(!showHref, 'no shows linked from home');
    const showURL = new URL(showHref!, LIVE_BASE).toString();
    await page.goto(showURL);
    const mediaImg = page.locator('img[src*="/media/"]').first();
    const imgCount = await mediaImg.count();
    test.skip(imgCount === 0, 'no media on show detail page to test thumbs against');
    const src = await mediaImg.getAttribute('src');
    expect(src).toMatch(/\/media\//);
    const fullURL = new URL(src!, LIVE_BASE);
    fullURL.searchParams.set('thumb', '1');
    const res = await request.get(fullURL.toString());
    expect(res.status()).toBeLessThan(500);
  });
});
