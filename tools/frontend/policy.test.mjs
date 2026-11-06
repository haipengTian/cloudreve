import assert from 'node:assert/strict';
import { test } from 'node:test';
import { existsSync } from 'node:fs';
import { readFile } from 'node:fs/promises';
import { runInNewContext } from 'node:vm';
import { resolve } from 'node:path';
import { pathToFileURL } from 'node:url';

test('ImageBed policy defaults confine storage and never contain a credential', async () => {
  const source = process.argv.find(x => x.startsWith('--source='))?.slice(9) ?? process.env.CLOUDREVE_FRONTEND_SOURCE;
  assert.ok(source, 'CLOUDREVE_FRONTEND_SOURCE must point to the pinned frontend checkout');
  const model = resolve(source, 'src/component/Admin/StoragePolicy/ImageBed/policy.ts');
  assert.ok(existsSync(model), 'ImageBed policy model must exist');
  const { createImageBedPolicy, configureImageBedPolicy } = await import(pathToFileURL(model));
  const policy = createImageBedPolicy();
  assert.equal(policy.type, 'imagebed');
  assert.equal(policy.server, 'https://img.thpvip.xyz');
  assert.equal(policy.bucket_name, 'cloudreve-test');
  assert.equal(policy.dir_name_rule, 'cloudreve-test/{uid}');
  assert.equal(policy.file_name_rule, '{uuid}{ext}');
  assert.equal(policy.secret_key, '');
  assert.equal(policy.access_key, undefined);
  assert.equal(policy.settings.imagebed_channel_name, undefined);
  assert.equal(policy.settings.relay, true);
  assert.equal(policy.settings.internal_proxy, true);
  assert.equal(policy.settings.chunk_size, 0);
  const updated = configureImageBedPolicy({ ...policy, bucket_name: 'new-root', settings: { relay: false, encryption: true } });
  assert.equal(updated.dir_name_rule, 'new-root/{uid}');
  assert.equal(updated.settings.relay, true);
  assert.equal(updated.settings.encryption, false);
  const fields = await readFile(resolve(source, 'src/component/Admin/StoragePolicy/ImageBed/ImageBedPolicyFields.tsx'), 'utf8');
  assert.ok(!fields.includes('title="上传渠道"') && !fields.includes('title="私密存储渠道名称"'));
});

test('retryable ImageBed upload errors preserve the existing session', async () => {
  const source = process.env.CLOUDREVE_FRONTEND_SOURCE;
  const text = await readFile(resolve(source, 'src/component/Uploader/core/uploader/base.ts'), 'utf8');
  const { default: ts } = await import(pathToFileURL(resolve(source, 'node_modules/typescript/lib/typescript.js')));
  const ast = ts.createSourceFile('base.ts', text, ts.ScriptTarget.Latest, true);
  const declaration = ast.statements.find(node => ts.isVariableStatement(node) && node.declarationList.declarations.some(d => d.name.getText(ast) === 'resumePolicy'));
  const policies = declaration.declarationList.declarations[0].initializer.elements.map(e => e.name.text);
  const cls = ast.statements.find(node => ts.isClassDeclaration(node));
  const method = cls.members.find(node => node.name?.getText(ast) === 'setError').getText(ast);
  const fixture = `const PolicyType=Object.fromEntries(${JSON.stringify(policies)}.map(x=>[x,x])); const resumePolicy=${JSON.stringify(policies)};
    class CanceledError extends Error {} class UploaderError extends Error { Retryable(){return true;} }
    const Status={error:1}; class Harness { task={policy:{type:'imagebed'}}; canceled=false; logger={warn(){}}; subscriber={onError(){}}; cancelUploadSession(){this.canceled=true;} ${method} }
    const h=new Harness(); h.setError(new UploaderError()); globalThis.retryCanceled=h.canceled; h.setError(new Error()); globalThis.failureCanceled=h.canceled;`;
  const context = {};
  runInNewContext(ts.transpileModule(fixture, { compilerOptions: { target: ts.ScriptTarget.ES2022 } }).outputText, context);
  assert.equal(context.retryCanceled, false);
  assert.equal(context.failureCanceled, true);
});
