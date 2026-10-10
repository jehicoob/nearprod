import test from 'node:test';
import assert from 'node:assert/strict';
import {stackActions, healthDescription} from '../../internal/webui/dist/ui/stack-state.js';

const stack = {activeMode:'dev'};
const container = (running, health = 'unchecked', hasHealthcheck = false, state = running ? 'running' : 'exited') => ({service:'api', running, health, hasHealthcheck, state});
const observed = (execution, containers = [], health = 'unchecked') => ({execution, containers, health});

test('running same-mode app offers apply/rebuild, not start', () => {
  const value = stackActions(stack, observed('running', [container(true)]), 'dev');
  assert.equal(value.start, false);
  assert.equal(value.apply, true);
  assert.equal(value.build, true);
  assert.equal(value.stop, true);
  assert.equal(value.restart, true);
  assert.match(value.startTitle, /Aplicar cambios/);
});
test('switching mode remains explicit and possible', () => {
  const value = stackActions(stack, observed('running', [container(true)]), 'verify');
  assert.equal(value.start, true);
  assert.equal(value.apply, true);
  assert.match(value.startTitle, /confirmación/);
});
test('stopped and absent apps cannot be stopped or restarted', () => {
  for (const value of [observed('stopped', [container(false)]), observed('not-created')]) {
    const actions = stackActions(stack, value, 'dev');
    assert.equal(actions.start, true);
    assert.equal(actions.stop, false);
    assert.equal(actions.restart, false);
    assert.equal(actions.apply, value.containers.length > 0);
  }
});
test('partial and failed apps retain useful recovery and stop actions', () => {
  for (const execution of ['partial', 'failed']) {
    const actions = stackActions(stack, observed(execution, [container(true), container(false)]), 'dev');
    assert.equal(actions.start, true);
    assert.equal(actions.stop, true);
    assert.equal(actions.apply, true);
  }
  assert.equal(stackActions(stack, observed('failed', [container(false)]), 'dev').stop, false);
});
test('booting and restart loops prevent duplicate starts while allowing stop', () => {
  for (const value of [observed('restarting', [container(false, 'unchecked', false, 'restarting')]), observed('running', [container(true, 'starting', true)], 'starting')]) {
    const actions = stackActions(stack, value, 'verify');
    assert.equal(actions.start, false);
    assert.equal(actions.apply, false);
    assert.equal(actions.build, false);
    assert.equal(actions.restart, false);
    assert.equal(actions.stop, true);
  }
});
test('unknown, disconnected, busy and purge-pending applications disable mutations', () => {
  for (const [value, unavailable] of [[undefined,false], [observed('unknown'),false], [observed('running',[container(true)]),true]]) {
    const actions = stackActions(stack, value, 'dev', unavailable);
    for (const action of ['start','stop','restart','apply','build']) assert.equal(actions[action],false);
  }
});
test('health copy distinguishes missing checks from checks without results', () => {
  assert.match(healthDescription('unchecked',[container(true)]).label, /Sin comprobación de salud configurada/);
  assert.deepEqual(healthDescription('unchecked',[container(true,'unchecked',true)]), {});
  assert.deepEqual(healthDescription('unchecked',[{service:'api',running:true}]), {});
  assert.deepEqual(healthDescription('unknown',[container(true)]), {});
});
test('mixed coverage is explicit without overriding health failures or warmup', () => {
  const mixed = [container(true,'healthy',true), {...container(true),service:'worker'}];
  assert.equal(healthDescription('unchecked',mixed).label,'Comprobación de salud incompleta');
  assert.match(healthDescription('unchecked',mixed).title,/worker/);
  assert.deepEqual(healthDescription('unhealthy',mixed), {});
  assert.deepEqual(healthDescription('starting',mixed), {});
  assert.deepEqual(healthDescription('unchecked',[container(false)]), {});
});
