import { test, expect } from '@playwright/test';
import {
  FLOWERSHOW_SERVICE_TOKEN,
  uniqueName,
} from './flowershow.helpers';

test.describe('Flowershow API', () => {
  test('health and contract discovery endpoints resolve', async ({ request }) => {
    const health = await request.get('/healthz');
    expect(health.ok()).toBeTruthy();
    const healthData = await health.json();
    expect(healthData.status).toBe('ok');
    expect(healthData.seed).toBe('0007-Flowershow');

    const contracts = await request.get('/v1/contracts');
    expect(contracts.ok()).toBeTruthy();
    const contractsText = await contracts.text();
    expect(contractsText).toContain(
      '/v1/contracts/0007-Flowershow/a-firstbloom',
    );

    const contract = await request.get(
      '/v1/contracts/0007-Flowershow/a-firstbloom',
    );
    expect(contract.ok()).toBeTruthy();
    const body = await contract.text();
    expect(body).toContain('"seed_agent_principles"');
    expect(body).toContain('"ui_surfaces"');
  });

  test('commands return structured errors for anonymous and authenticated callers', async ({
    request,
  }) => {
    const anonymous = await request.post(
      '/v1/commands/0007-Flowershow/shows.create',
      {
        data: { name: 'Unauthorized Show' },
      },
    );
    expect(anonymous.status()).toBe(401);
    expect(await anonymous.json()).toMatchObject({
      error: {
        code: 'unauthorized',
        auth_mode: 'anonymous',
      },
    });

    const authenticated = await request.post(
      '/v1/commands/0007-Flowershow/shows.create',
      {
        data: '{"name":"bad"',
        headers: {
          Authorization: `Bearer ${FLOWERSHOW_SERVICE_TOKEN}`,
          'Content-Type': 'application/json',
        },
      },
    );
    expect(authenticated.status()).toBe(400);
    expect(await authenticated.json()).toMatchObject({
      error: {
        code: 'invalid_json',
        auth_mode: 'service_token',
        contract_ref: '/v1/contracts/0007-Flowershow/a-firstbloom',
      },
    });
  });

  test('service token can create governed show data and inspect private by-id projections', async ({
    request,
  }) => {
    const showName = uniqueName('API Runtime Show');

    const createShow = await request.post(
      '/v1/commands/0007-Flowershow/shows.create',
      {
        data: {
          input: {
            organization_id: 'org_demo1',
            name: showName,
            season: '2026',
          },
          runtime_context: {
            assistant_goal: 'create a governed schedule scaffold',
            source_excerpt: 'Use schedule notes as operator-only authoring context.',
          },
        },
        headers: {
          Authorization: `Bearer ${FLOWERSHOW_SERVICE_TOKEN}`,
        },
      },
    );
    expect(createShow.status()).toBe(201);
    const show = await createShow.json();
    expect(show.name).toBe(showName);

    const schedule = await request.post(
      '/v1/commands/0007-Flowershow/schedules.upsert',
      {
        data: {
          show_id: show.id,
          notes: 'OJES governs unless the local schedule is narrower.',
        },
        headers: {
          Authorization: `Bearer ${FLOWERSHOW_SERVICE_TOKEN}`,
        },
      },
    );
    expect(schedule.status()).toBe(200);
    const scheduleBody = await schedule.json();

    const division = await request.post(
      '/v1/commands/0007-Flowershow/divisions.create',
      {
        data: {
          show_schedule_id: scheduleBody.id,
          code: 'I',
          title: 'API Division',
          domain: 'horticulture',
          sort_order: 1,
        },
        headers: {
          Authorization: `Bearer ${FLOWERSHOW_SERVICE_TOKEN}`,
        },
      },
    );
    expect(division.status()).toBe(201);
    const divisionBody = await division.json();

    const section = await request.post(
      '/v1/commands/0007-Flowershow/sections.create',
      {
        data: {
          division_id: divisionBody.id,
          code: 'A',
          title: 'API Section',
          sort_order: 1,
        },
        headers: {
          Authorization: `Bearer ${FLOWERSHOW_SERVICE_TOKEN}`,
        },
      },
    );
    expect(section.status()).toBe(201);
    const sectionBody = await section.json();

    const createClass = await request.post(
      '/v1/commands/0007-Flowershow/classes.create',
      {
        data: {
          section_id: sectionBody.id,
          class_number: '101',
          title: 'API Bloom',
          domain: 'horticulture',
          specimen_count: 1,
        },
        headers: {
          Authorization: `Bearer ${FLOWERSHOW_SERVICE_TOKEN}`,
        },
      },
    );
    expect(createClass.status()).toBe(201);
    const classBody = await createClass.json();

    const createEntry = await request.post(
      '/v1/commands/0007-Flowershow/entries.create',
      {
        data: {
          show_id: show.id,
          class_id: classBody.id,
          person_id: 'person_01',
          name: 'API Entry',
        },
        headers: {
          Authorization: `Bearer ${FLOWERSHOW_SERVICE_TOKEN}`,
        },
      },
    );
    expect(createEntry.status()).toBe(201);
    const entry = await createEntry.json();

    const uploadMedia = await request.post(
      `/v1/commands/0007-Flowershow/entries/${entry.id}/media.upload`,
      {
        multipart: {
          media: {
            name: 'api-upload.jpg',
            mimeType: 'image/jpeg',
            buffer: Buffer.from('api upload bytes'),
          },
        },
        headers: {
          Authorization: `Bearer ${FLOWERSHOW_SERVICE_TOKEN}`,
        },
      },
    );
    expect(uploadMedia.status()).toBe(201);
    const uploadBody = await uploadMedia.json();
    expect(uploadBody.entry_id).toBe(entry.id);
    expect(uploadBody.media).toHaveLength(1);

    const workspace = await request.get(
      `/v1/projections/0007-Flowershow/shows/${show.id}/workspace`,
      {
        headers: {
          Authorization: `Bearer ${FLOWERSHOW_SERVICE_TOKEN}`,
        },
      },
    );
    expect(workspace.ok()).toBeTruthy();
    const workspaceText = await workspace.text();
    expect(workspaceText).toContain(showName);
    expect(workspaceText).not.toContain('assistant_goal');

    const board = await request.get(
      `/v1/projections/0007-Flowershow/shows/${show.id}/board`,
      {
        headers: {
          Authorization: `Bearer ${FLOWERSHOW_SERVICE_TOKEN}`,
        },
      },
    );
    expect(board.ok()).toBeTruthy();
    const boardText = await board.text();
    expect(boardText).toContain('API Entry');

    const classDetail = await request.get(
      `/v1/projections/0007-Flowershow/classes/${classBody.id}`,
      {
        headers: {
          Authorization: `Bearer ${FLOWERSHOW_SERVICE_TOKEN}`,
        },
      },
    );
    expect(classDetail.ok()).toBeTruthy();
    expect(await classDetail.json()).toMatchObject({
      class: {
        id: classBody.id,
        title: 'API Bloom',
      },
      show: {
        id: show.id,
      },
    });

    const suppress = await request.post(
      '/v1/commands/0007-Flowershow/entries.set_visibility',
      {
        data: {
          id: entry.id,
          suppressed: true,
        },
        headers: {
          Authorization: `Bearer ${FLOWERSHOW_SERVICE_TOKEN}`,
        },
      },
    );
    expect(suppress.status()).toBe(200);

    const anonymousEntry = await request.get(
      `/v1/projections/0007-Flowershow/entries/${entry.id}`,
    );
    expect(anonymousEntry.status()).toBe(404);

    const privateEntry = await request.get(
      `/v1/projections/0007-Flowershow/entries/${entry.id}`,
      {
        headers: {
          Authorization: `Bearer ${FLOWERSHOW_SERVICE_TOKEN}`,
        },
      },
    );
    expect(privateEntry.ok()).toBeTruthy();
    expect(await privateEntry.json()).toMatchObject({
      entry: {
        id: entry.id,
        suppressed: true,
      },
      media: [
        {
          id: uploadBody.media[0].id,
        },
      ],
      person: {
        first_name: 'Margaret',
      },
    });
  });
});
