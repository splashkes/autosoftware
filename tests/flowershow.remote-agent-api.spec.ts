import { test, expect } from '@playwright/test';
import {
  FLOWERSHOW_REMOTE_E2E,
  FLOWERSHOW_SERVICE_TOKEN,
} from './flowershow.helpers';

test.describe('Flowershow Remote Agent API', () => {
  test.skip(
    !FLOWERSHOW_REMOTE_E2E,
    'Set FLOWERSHOW_REMOTE_E2E=1 to run remote agent API smoke coverage.',
  );
  test.skip(
    !process.env.FLOWERSHOW_SERVICE_TOKEN,
    'Set FLOWERSHOW_SERVICE_TOKEN to run remote agent API smoke coverage.',
  );

  test('remote contracts, workspace, and structured errors are reachable', async ({
    request,
  }) => {
    const contracts = await request.get('/v1/contracts');
    expect(contracts.ok()).toBeTruthy();

    const workspace = await request.get(
      '/v1/projections/0007-Flowershow/shows/show_spring2025/workspace',
      {
        headers: {
          Authorization: `Bearer ${FLOWERSHOW_SERVICE_TOKEN}`,
        },
      },
    );
    expect(workspace.ok()).toBeTruthy();
    expect(await workspace.text()).toContain('Spring Rose Show 2025');

    const invalid = await request.post(
      '/v1/commands/0007-Flowershow/shows.create',
      {
        data: '{"name":"bad"',
        headers: {
          Authorization: `Bearer ${FLOWERSHOW_SERVICE_TOKEN}`,
          'Content-Type': 'application/json',
        },
      },
    );
    expect(invalid.status()).toBe(400);
    expect(await invalid.json()).toMatchObject({
      error: {
        code: 'invalid_json',
        auth_mode: 'service_token',
      },
    });
  });
});
