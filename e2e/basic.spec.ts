import { test, expect, APIRequestContext } from '@playwright/test';
import * as fs from 'fs';
import * as os from 'os';
import * as path from 'path';

async function inject(request: APIRequestContext, line: string, src = 'test.log') {
  const res = await request.post(`/inject?src=${encodeURIComponent(src)}`, {
    data: line,
    headers: { 'Content-Type': 'text/plain' },
  });
  expect(res.status()).toBe(204);
}

async function findStatusNumbers(page: Parameters<typeof test>[1] extends { page: infer P } ? P : never) {
  const status = page.locator('#find-status');
  await expect(status).toHaveText(/\d+ \/ \d+/);
  const [current, total] = (await status.innerText()).split(' / ').map(Number);
  return { current, total };
}

test.beforeEach(async ({ request }) => {
  await request.post('/reset');
});

test('renders message, badge, src, and timestamp', async ({ page, request }) => {
  await inject(request, '{"time":"2024-01-15T10:00:00Z","level":"info","msg":"hello world"}');
  await page.goto('/');

  const entry = page.locator('.entry').first();
  await expect(entry.locator('.msg')).toHaveText('hello world');
  await expect(entry.locator('.badge')).toHaveText('INFO');   // levels are uppercased
  await expect(entry.locator('.src')).toContainText('test.log');
  await expect(entry.locator('.ts')).not.toBeEmpty();
});

test('src column has inline color and fruit emoji', async ({ page, request }) => {
  await inject(request, '{"msg":"hi"}', 'app.log');
  await page.goto('/');

  const src = page.locator('.entry .src').first();
  await expect(src).toContainText('app.log');
  const color = await src.evaluate(el => (el as HTMLElement).style.color);
  expect(color).toBeTruthy();
  // emoji precedes the source name
  const text = await src.innerText();
  expect([...text][0]).not.toMatch(/[a-zA-Z0-9]/);
});

test('find bar opens with Cmd+F', async ({ page, request }) => {
  await inject(request, '{"msg":"something"}');
  await page.goto('/');

  await expect(page.locator('#find-bar')).toHaveClass(/hidden/);
  await page.keyboard.press('Meta+f');
  await expect(page.locator('#find-bar')).not.toHaveClass(/hidden/);
});

test('find starts at last (newest) match', async ({ page, request }) => {
  for (let i = 1; i <= 5; i++) {
    await inject(request, `{"msg":"needle ${i}"}`);
  }
  await page.goto('/');

  await page.keyboard.press('Meta+f');
  await page.fill('#find-input', 'needle');

  // current == total means we are at the last match
  const { current, total } = await findStatusNumbers(page);
  expect(current).toBe(total);
});

test('Enter navigates to older (previous) match', async ({ page, request }) => {
  for (let i = 1; i <= 3; i++) {
    await inject(request, `{"msg":"needle ${i}"}`);
  }
  await page.goto('/');

  await page.keyboard.press('Meta+f');
  await page.fill('#find-input', 'needle');

  const { current: start, total } = await findStatusNumbers(page);
  expect(start).toBe(total); // starts at last

  await page.keyboard.press('Enter');
  await expect(page.locator('#find-status')).toHaveText(`${start - 1} / ${total}`);

  await page.keyboard.press('Enter');
  await expect(page.locator('#find-status')).toHaveText(`${start - 2} / ${total}`);
});

test('Shift+Enter navigates to newer (next) match', async ({ page, request }) => {
  for (let i = 1; i <= 3; i++) {
    await inject(request, `{"msg":"needle ${i}"}`);
  }
  await page.goto('/');

  await page.keyboard.press('Meta+f');
  await page.fill('#find-input', 'needle');

  const { current: start, total } = await findStatusNumbers(page);
  await page.keyboard.press('Enter');            // → start-1 / total
  await page.keyboard.press('Shift+Enter');      // → start / total
  await expect(page.locator('#find-status')).toHaveText(`${start} / ${total}`);
});

test('open files dialog closes a file and removes its entries', async ({ page, request }) => {
  const dir = fs.mkdtempSync(path.join(os.tmpdir(), 'jsonlv-'));
  const file = path.join(dir, 'closeme.log');
  fs.writeFileSync(file, '{"msg":"from file"}\n');
  await request.post(`/add-file?path=${encodeURIComponent(file)}`);
  await inject(request, '{"msg":"from other"}', 'other.log');
  await page.goto('/');
  await expect(page.locator('.entry .msg', { hasText: 'from file' })).toBeVisible();

  await page.evaluate(() => (window as any).openFilesDialog());
  const row = page.locator('#files-list li', { hasText: 'closeme.log' });
  await expect(row).toContainText(file);
  await row.getByRole('button', { name: 'Schließen' }).click();

  await expect(page.locator('#files-list')).toContainText('Keine Dateien geöffnet.');
  await expect(page.locator('.entry .msg', { hasText: 'from file' })).toHaveCount(0);
  await expect(page.locator('.entry .msg', { hasText: 'from other' })).toBeVisible();

  await page.keyboard.press('Escape');
  await expect(page.locator('#files-modal')).toBeHidden();
});
