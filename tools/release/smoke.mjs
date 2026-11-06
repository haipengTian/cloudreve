import assert from 'node:assert/strict';
import { randomBytes } from 'node:crypto';
import { readFile, writeFile } from 'node:fs/promises';

const [base, stateFile, phase = 'create'] = process.argv.slice(2);
assert.ok(base?.startsWith('http://127.0.0.1:') && stateFile);
let token;
async function api(route, method, body) {
  const binary = Buffer.isBuffer(body);
  const response = await fetch(`${base}/api/v4${route}`, {
    method,
    headers: { 'Content-Type': binary ? 'application/octet-stream' : 'application/json', ...(token ? { Authorization: `Bearer ${token}` } : {}) },
    body: body === undefined ? undefined : binary ? body : JSON.stringify(body),
  });
  const result = await response.json();
  assert.equal(response.status, 200, `${method} ${route} HTTP status`);
  assert.equal(result.code, 0, `${method} ${route}: ${result.msg || 'API failed'}`);
  return result.data;
}
const state = phase === 'create'
  ? { email: 'release-smoke@example.com', password: randomBytes(24).toString('hex'), uri: 'cloudreve://my/release-smoke.html' }
  : JSON.parse(await readFile(stateFile, 'utf8'));
if (phase === 'create') {
  await writeFile(stateFile, JSON.stringify(state), { mode: 0o600 });
  await api('/user', 'POST', { email: state.email, password: state.password, language: 'zh-CN' });
}
const login = await api('/session/token', 'POST', { email: state.email, password: state.password });
token = login.token.access_token;
assert.ok(token, 'login must issue an access token');
const content = Buffer.from('<!doctype html><meta charset="utf-8"><h1>镜像持久化验收</h1>\n');
if (phase === 'create') {
  const upload = await api('/file/upload', 'PUT', { uri: state.uri, size: content.length, mime_type: 'text/html' });
  await api(`/file/upload/${upload.session_id}/0`, 'POST', content);
}
const urls = await api('/file/url', 'POST', { uris: [state.uri], download: true });
assert.equal(urls.urls.length, 1);
const signed = new URL(urls.urls[0].url, base);
// The container's configured public port may differ from Docker's random host
// port. Its local content signature covers the path/query, not this CI mapping.
const download = `${base}${signed.pathname}${signed.search}`;
const response = await fetch(download);
assert.equal(response.status, 200);
assert.deepEqual(Buffer.from(await response.arrayBuffer()), content);
const ranged = await fetch(download, { headers: { Range: 'bytes=0-14' } });
assert.equal(ranged.status, 206);
assert.deepEqual(Buffer.from(await ranged.arrayBuffer()), content.subarray(0, 15));
console.log(`${phase}: account login, file download bytes and Range passed`);
