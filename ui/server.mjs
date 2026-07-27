import http from 'node:http';
import { readFile, stat, writeFile, copyFile, access } from 'node:fs/promises';
import { createReadStream } from 'node:fs';
import path from 'node:path';
import os from 'node:os';
import { fileURLToPath } from 'node:url';
import { spawn } from 'node:child_process';

const __dirname = path.dirname(fileURLToPath(import.meta.url));
const ROOT = process.env.LOCAL_INFRA_ROOT || path.resolve(__dirname, '..');
const PROJECTS_FILE = process.env.LOCAL_INFRA_PROJECTS || path.join(ROOT, 'projects.yml');
const DIST_DIR = path.join(__dirname, 'dist');
const PUBLIC_DIR = path.join(__dirname, 'public');
const HOST = process.env.LOCAL_INFRA_UI_HOST || '0.0.0.0';
const PORT = Number(process.env.LOCAL_INFRA_UI_PORT || 5055);
const MAX_JOB_OUTPUT = 160_000;
const VALID_ACTIONS = new Set(['start', 'stop', 'down', 'restart']);
const RUNNABLE_RUNNERS = new Set(['docker-compose']);

const jobs = new Map();

function nowStamp() {
  return new Date().toISOString().replace(/[-:]/g, '').replace(/\.\d+Z$/, 'Z');
}

function parseScalar(value) {
  const trimmed = value.trim();
  if ((trimmed.startsWith('"') && trimmed.endsWith('"')) || (trimmed.startsWith("'") && trimmed.endsWith("'"))) {
    return trimmed.slice(1, -1).replaceAll('\\"', '"');
  }
  if (trimmed === 'true') return true;
  if (trimmed === 'false') return false;
  if (trimmed === 'null') return null;
  return trimmed;
}

function quoteYaml(value = '') {
  return `"${String(value).replaceAll('\\', '\\\\').replaceAll('"', '\\"')}"`;
}

function stripComment(line) {
  let quote = null;
  for (let i = 0; i < line.length; i += 1) {
    const char = line[i];
    if ((char === '"' || char === "'") && line[i - 1] !== '\\') quote = quote === char ? null : quote || char;
    if (char === '#' && !quote) return line.slice(0, i);
  }
  return line;
}

function expandHome(value) {
  if (!value) return value;
  if (value === '~') return os.homedir();
  if (value.startsWith('~/')) return path.join(os.homedir(), value.slice(2));
  return value;
}

function parseKnownConfig(raw) {
  const config = { groups: {}, projects: {} };
  let top = null;
  let currentId = null;
  let currentSection = null;

  for (const originalLine of raw.split(/\r?\n/)) {
    const line = stripComment(originalLine).replace(/\s+$/, '');
    if (!line.trim()) continue;
    const indent = line.match(/^\s*/)[0].length;
    const trimmed = line.trim();

    if (indent === 0 && trimmed.endsWith(':')) {
      const key = trimmed.slice(0, -1);
      if (key === 'groups' || key === 'projects') top = key;
      currentId = null;
      currentSection = null;
      continue;
    }

    if (!top) continue;

    if (indent === 2 && trimmed.endsWith(':')) {
      currentId = trimmed.slice(0, -1);
      config[top][currentId] = { id: currentId };
      currentSection = null;
      continue;
    }

    if (!currentId) continue;

    if (indent === 4) {
      const [key, ...rest] = trimmed.split(':');
      const value = rest.join(':').trim();
      if (!value) {
        currentSection = key;
        config[top][currentId][key] = ['components', 'profiles'].includes(key) ? [] : {};
      } else {
        currentSection = null;
        config[top][currentId][key] = parseScalar(value);
      }
      continue;
    }

    if (indent === 6 && currentSection) {
      if (trimmed.startsWith('- ')) {
        if (!Array.isArray(config[top][currentId][currentSection])) config[top][currentId][currentSection] = [];
        config[top][currentId][currentSection].push(parseScalar(trimmed.slice(2)));
        continue;
      }
      const [key, ...rest] = trimmed.split(':');
      const value = rest.join(':').trim();
      if (!Array.isArray(config[top][currentId][currentSection])) {
        config[top][currentId][currentSection][key] = parseScalar(value);
      }
    }
  }

  if (!Object.keys(config.groups).length) {
    // Backwards-compatible synthetic groups for older flat projects.yml files.
    for (const project of Object.values(config.projects)) {
      config.groups[project.id] = {
        id: project.id,
        label: project.label || project.id.replaceAll('_', ' '),
        description: 'Proyecto individual registrado sin grupo.',
        components: [project.id],
        quick_links: project.urls || {},
      };
    }
  }

  return config;
}

function serializeConfig(config) {
  const lines = [];
  lines.push('groups:');
  for (const group of Object.values(config.groups)) {
    lines.push(`  ${group.id}:`);
    if (group.label) lines.push(`    label: ${quoteYaml(group.label)}`);
    if (group.description) lines.push(`    description: ${quoteYaml(group.description)}`);
    lines.push('    components:');
    for (const component of group.components || []) lines.push(`      - ${component}`);
    const quickLinks = group.quick_links || {};
    if (Object.keys(quickLinks).length) {
      lines.push('    quick_links:');
      for (const [key, value] of Object.entries(quickLinks)) lines.push(`      ${key}: ${quoteYaml(value)}`);
    }
    lines.push('');
  }

  lines.push('projects:');
  for (const project of Object.values(config.projects)) {
    lines.push(`  ${project.id}:`);
    for (const key of ['label', 'role', 'runner', 'source_type', 'git_url', 'path', 'project_name', 'type']) {
      if (project[key]) lines.push(`    ${key}: ${quoteYaml(project[key])}`);
    }
    if (project.trusted !== undefined) lines.push(`    trusted: ${project.trusted ? 'true' : 'false'}`);
    if (Array.isArray(project.profiles) && project.profiles.length) {
      lines.push('    profiles:');
      for (const profile of project.profiles) lines.push(`      - ${profile}`);
    }
    const urls = project.urls || {};
    if (Object.keys(urls).length) {
      lines.push('    urls:');
      for (const [key, value] of Object.entries(urls)) lines.push(`      ${key}: ${quoteYaml(value)}`);
    }
    lines.push('');
  }
  return `${lines.join('\n').replace(/\n{3,}/g, '\n\n').trim()}\n`;
}

async function loadConfig() {
  const raw = await readFile(PROJECTS_FILE, 'utf8');
  return parseKnownConfig(raw);
}

async function saveConfig(config) {
  const backup = `${PROJECTS_FILE}.bak-${nowStamp()}`;
  await copyFile(PROJECTS_FILE, backup);
  await writeFile(PROJECTS_FILE, serializeConfig(config), 'utf8');
  return backup;
}

function normalizeProject(project) {
  return {
    ...project,
    id: project.id,
    label: project.label || project.id.replaceAll('_', ' '),
    path: project.path,
    resolvedPath: expandHome(project.path),
    project_name: project.project_name || project.id,
    runner: project.runner || 'docker-compose',
    type: project.type || 'docker-compose',
    role: project.role || 'service',
    urls: project.urls || {},
    profiles: project.profiles || [],
  };
}

function deriveQuickLinks(group, components) {
  const quick = { ...(group.quick_links || {}) };
  for (const component of components) {
    if (component.role === 'frontend') {
      if (!quick.frontend && component.urls.frontend) quick.frontend = component.urls.frontend;
      if (!quick.frontend && component.urls.app) quick.frontend = component.urls.app;
    }
    if (component.role === 'backend') {
      if (!quick.backend_docs && component.urls.docs) quick.backend_docs = component.urls.docs;
      if (!quick.api && component.urls.api) quick.api = component.urls.api;
    }
  }
  return quick;
}

function groupStatus(components) {
  const runnable = components.filter((component) => component.runner === 'docker-compose');
  if (!runnable.length) return 'manual';
  const running = runnable.filter((component) => component.status === 'running').length;
  if (running === runnable.length) return 'running';
  if (running > 0) return 'partial';
  if (runnable.some((component) => component.status === 'unknown')) return 'unknown';
  return 'stopped';
}

function execBuffered(command, args, options = {}) {
  return new Promise((resolve) => {
    const child = spawn(command, args, {
      cwd: options.cwd || ROOT,
      env: { ...process.env, ...(options.env || {}) },
      shell: false,
    });
    let stdout = '';
    let stderr = '';
    child.stdout.on('data', (data) => { stdout += data.toString(); });
    child.stderr.on('data', (data) => { stderr += data.toString(); });
    child.on('error', (error) => resolve({ code: -1, stdout, stderr: `${stderr}${error.message}` }));
    child.on('close', (code) => resolve({ code, stdout, stderr }));
  });
}

function parseJsonLines(output) {
  return output.split(/\r?\n/).map((line) => line.trim()).filter(Boolean).map((line) => {
    try { return JSON.parse(line); } catch { return null; }
  }).filter(Boolean);
}

function normalizeContainer(container) {
  return {
    id: container.ID,
    name: container.Names,
    image: container.Image,
    state: container.State,
    status: container.Status,
    health: container.HealthStatus,
    ports: container.Ports,
  };
}

async function getProjectStatus(projectInput) {
  const project = normalizeProject(projectInput);
  if (project.runner !== 'docker-compose') {
    return { ...project, exists: false, status: project.runner === 'external' ? 'external' : 'manual', running: 0, total: 0, containers: [] };
  }

  const result = await execBuffered('docker', [
    'ps', '-a', '--filter', `label=com.docker.compose.project=${project.project_name}`, '--format', '{{json .}}',
  ]);

  if (result.code !== 0) {
    return { ...project, exists: false, status: 'unknown', error: result.stderr || result.stdout || 'No se pudo consultar Docker.', containers: [], running: 0, total: 0 };
  }

  const containers = parseJsonLines(result.stdout).map(normalizeContainer);
  const running = containers.filter((container) => container.state === 'running').length;
  let status = 'stopped';
  if (containers.length === 0) status = 'not_created';
  else if (running === containers.length) status = 'running';
  else if (running > 0) status = 'partial';

  return { ...project, exists: containers.length > 0, status, running, total: containers.length, containers };
}

async function getDockerSummary() {
  const [context, version] = await Promise.all([
    execBuffered('docker', ['context', 'show']),
    execBuffered('docker', ['version', '--format', '{{json .}}']),
  ]);
  let parsedVersion = null;
  try { parsedVersion = JSON.parse(version.stdout || '{}'); } catch { parsedVersion = null; }
  return {
    context: context.stdout.trim() || 'unknown',
    clientVersion: parsedVersion?.Client?.Version,
    serverVersion: parsedVersion?.Server?.Version,
    dockerOk: context.code === 0 && version.code === 0,
    error: context.stderr || version.stderr || null,
  };
}

async function getDashboard() {
  const config = await loadConfig();
  const projectStatuses = await Promise.all(Object.values(config.projects).map(getProjectStatus));
  const projectMap = Object.fromEntries(projectStatuses.map((project) => [project.id, project]));
  const groups = Object.values(config.groups).map((group) => {
    const components = (group.components || []).map((id) => projectMap[id]).filter(Boolean);
    return {
      ...group,
      label: group.label || group.id.replaceAll('_', ' '),
      components,
      quick_links: deriveQuickLinks(group, components),
      status: groupStatus(components),
      running: components.reduce((sum, component) => sum + (component.running || 0), 0),
      total: components.reduce((sum, component) => sum + (component.total || 0), 0),
    };
  });
  return { config, projects: projectStatuses, groups };
}

function commandForAction(action, project) {
  const base = ['compose', '-p', project.project_name];
  if (action === 'start') return [...base, 'up', '-d', '--build', '--remove-orphans'];
  if (action === 'stop') return [...base, 'stop'];
  if (action === 'down') return [...base, 'down', '--remove-orphans'];
  if (action === 'restart') return [...base, 'restart'];
  throw new Error(`Acción no soportada: ${action}`);
}

function redactOutput(output) {
  if (output.length <= MAX_JOB_OUTPUT) return output;
  return `${output.slice(-MAX_JOB_OUTPUT)}\n\n[output truncado a ${MAX_JOB_OUTPUT} caracteres]`;
}

async function assertProjectRunnable(project) {
  if (!RUNNABLE_RUNNERS.has(project.runner)) {
    const error = new Error(`El componente ${project.id} usa runner '${project.runner}' y no se puede ejecutar automáticamente.`);
    error.statusCode = 400;
    throw error;
  }
  try {
    const projectPath = project.resolvedPath;
    const stats = await stat(projectPath);
    if (!stats.isDirectory()) throw new Error('No es un directorio.');
  } catch (error) {
    const wrapped = new Error(`No se encontró el path del proyecto: ${project.resolvedPath}. ${error.message}`);
    wrapped.statusCode = 400;
    throw wrapped;
  }
}

function createBaseJob({ action, targetId, targetType, command = '' }) {
  const id = `${Date.now().toString(36)}-${Math.random().toString(36).slice(2, 8)}`;
  const job = { id, targetId, targetType, action, command, status: 'running', exitCode: null, output: '', startedAt: new Date().toISOString(), finishedAt: null };
  jobs.set(id, job);
  return job;
}

function appendJob(job, chunk) {
  job.output = redactOutput(`${job.output}${chunk.toString()}`);
}

async function createProjectJob(projectInput, action) {
  if (!VALID_ACTIONS.has(action)) {
    const error = new Error('Acción inválida.');
    error.statusCode = 400;
    throw error;
  }
  const project = normalizeProject(projectInput);
  await assertProjectRunnable(project);
  const args = commandForAction(action, project);
  const job = createBaseJob({ action, targetId: project.id, targetType: 'project', command: `docker ${args.join(' ')}` });
  const child = spawn('docker', args, { cwd: project.resolvedPath, env: process.env, shell: false });
  child.stdout.on('data', (data) => appendJob(job, data));
  child.stderr.on('data', (data) => appendJob(job, data));
  child.on('error', (error) => { appendJob(job, `\n${error.message}\n`); job.status = 'failed'; job.exitCode = -1; job.finishedAt = new Date().toISOString(); });
  child.on('close', (code) => { job.exitCode = code; job.status = code === 0 ? 'succeeded' : 'failed'; job.finishedAt = new Date().toISOString(); });
  return job;
}

async function runCommandIntoJob(job, project, args) {
  return new Promise((resolve) => {
    appendJob(job, `\n$ cd ${project.resolvedPath}\n$ docker ${args.join(' ')}\n`);
    const child = spawn('docker', args, { cwd: project.resolvedPath, env: process.env, shell: false });
    child.stdout.on('data', (data) => appendJob(job, data));
    child.stderr.on('data', (data) => appendJob(job, data));
    child.on('error', (error) => { appendJob(job, `\n${error.message}\n`); resolve(-1); });
    child.on('close', (code) => resolve(code));
  });
}

async function createGroupJob(group, projects, action) {
  if (!VALID_ACTIONS.has(action)) {
    const error = new Error('Acción inválida.');
    error.statusCode = 400;
    throw error;
  }
  const runnable = (group.components || []).map((id) => projects[id]).filter(Boolean).map(normalizeProject).filter((project) => project.runner === 'docker-compose');
  if (!runnable.length) {
    const error = new Error('Este producto no tiene componentes Docker Compose ejecutables.');
    error.statusCode = 400;
    throw error;
  }
  const ordered = ['stop', 'down'].includes(action) ? [...runnable].reverse() : runnable;
  const job = createBaseJob({ action, targetId: group.id, targetType: 'group', command: `${action} ${ordered.map((project) => project.id).join(', ')}` });

  queueMicrotask(async () => {
    let failed = false;
    for (const project of ordered) {
      try {
        await assertProjectRunnable(project);
        const code = await runCommandIntoJob(job, project, commandForAction(action, project));
        if (code !== 0) {
          failed = true;
          appendJob(job, `\n[${project.id}] finalizó con código ${code}. Se detiene la ejecución del grupo.\n`);
          break;
        }
      } catch (error) {
        failed = true;
        appendJob(job, `\n[${project.id}] ${error.message}\n`);
        break;
      }
    }
    job.exitCode = failed ? 1 : 0;
    job.status = failed ? 'failed' : 'succeeded';
    job.finishedAt = new Date().toISOString();
  });

  return job;
}

async function getProjectLogs(projectInput, tail = 160) {
  const project = normalizeProject(projectInput);
  if (project.runner !== 'docker-compose') return { code: 400, output: `El runner ${project.runner} no tiene logs de Docker Compose.`, stderr: '' };
  const safeTail = String(Math.min(Math.max(Number(tail) || 160, 20), 1000));
  const result = await execBuffered('docker', ['compose', '-p', project.project_name, 'logs', '--tail', safeTail], { cwd: project.resolvedPath });
  return { code: result.code, output: result.stdout || result.stderr, stderr: result.stderr };
}

async function pathExists(filePath) {
  try { await access(filePath); return true; } catch { return false; }
}

async function detectProject(inputPath) {
  const resolvedPath = expandHome(inputPath || '');
  const result = {
    inputPath,
    resolvedPath,
    exists: false,
    isDirectory: false,
    runnerSuggestion: 'manual',
    typeSuggestion: 'manual',
    roleSuggestion: 'service',
    files: {},
    warnings: [],
  };
  if (!resolvedPath) {
    result.warnings.push('Ingresa una ruta local para detectar el proyecto.');
    return result;
  }
  try {
    const stats = await stat(resolvedPath);
    result.exists = true;
    result.isDirectory = stats.isDirectory();
  } catch {
    result.warnings.push('La ruta no existe dentro del contenedor de local-infra. Verifica que esté bajo ~/Trabajo o que esté montada.');
    return result;
  }

  const checks = {
    compose: ['docker-compose.yml', 'compose.yml', 'compose.yaml'],
    dockerfile: ['Dockerfile'],
    packageJson: ['package.json'],
    vite: ['vite.config.ts', 'vite.config.js'],
    next: ['next.config.js', 'next.config.mjs', 'next.config.ts'],
    laravel: ['artisan', 'composer.json'],
    python: ['pyproject.toml', 'requirements.txt'],
    envExample: ['.env.example'],
  };
  for (const [key, candidates] of Object.entries(checks)) {
    result.files[key] = [];
    for (const candidate of candidates) {
      if (await pathExists(path.join(resolvedPath, candidate))) result.files[key].push(candidate);
    }
  }

  const lower = resolvedPath.toLowerCase();
  if (result.files.compose.length) result.runnerSuggestion = 'docker-compose';
  else if (result.files.dockerfile.length) result.runnerSuggestion = 'dockerfile';
  else result.runnerSuggestion = 'manual';

  if (result.files.next.length) result.typeSuggestion = 'nextjs';
  else if (result.files.vite.length) result.typeSuggestion = 'react-vite';
  else if (result.files.packageJson.length) result.typeSuggestion = 'node';
  else if (result.files.laravel.length === 2) result.typeSuggestion = 'laravel-api';
  else if (result.files.python.length) result.typeSuggestion = 'python-api';
  else result.typeSuggestion = result.runnerSuggestion;

  if (lower.includes('front')) result.roleSuggestion = 'frontend';
  else if (lower.includes('api') || lower.includes('back')) result.roleSuggestion = 'backend';

  if (result.runnerSuggestion !== 'docker-compose') {
    result.warnings.push('No se encontró docker-compose.yml; quedará como manual hasta dockerizar o configurar un runner.');
  }
  return result;
}

function ensureSlug(slug, label = 'slug') {
  if (!/^[a-z0-9][a-z0-9_-]*$/.test(slug || '')) {
    const error = new Error(`${label} inválido. Usa minúsculas, números, guiones o underscores.`);
    error.statusCode = 400;
    throw error;
  }
}

function normalizeUrlMap(urls = {}) {
  return Object.fromEntries(Object.entries(urls).filter(([, value]) => String(value || '').trim()).map(([key, value]) => [key, String(value).trim()]));
}

async function createProduct(payload) {
  ensureSlug(payload.slug, 'Product slug');
  const config = await loadConfig();
  if (config.groups[payload.slug]) {
    const error = new Error('Ya existe un producto/grupo con ese slug.');
    error.statusCode = 409;
    throw error;
  }
  if (!Array.isArray(payload.components) || !payload.components.length) {
    const error = new Error('Agrega al menos un componente.');
    error.statusCode = 400;
    throw error;
  }

  const componentIds = [];
  for (const rawComponent of payload.components) {
    const id = rawComponent.id || `${payload.slug}_${rawComponent.role || 'service'}`;
    ensureSlug(id, 'Component id');
    if (config.projects[id]) {
      const error = new Error(`Ya existe un componente con id ${id}.`);
      error.statusCode = 409;
      throw error;
    }
    const runner = rawComponent.runner || 'manual';
    const project = {
      id,
      label: rawComponent.label || rawComponent.role || id,
      role: rawComponent.role || 'service',
      runner,
      source_type: rawComponent.source_type || 'local-folder',
      path: rawComponent.path || '',
      project_name: rawComponent.project_name || id,
      type: rawComponent.type || runner,
      urls: normalizeUrlMap(rawComponent.urls || {}),
    };
    if (rawComponent.git_url) project.git_url = rawComponent.git_url;
    if (rawComponent.trusted !== undefined) project.trusted = Boolean(rawComponent.trusted);
    config.projects[id] = project;
    componentIds.push(id);
  }

  const componentObjects = componentIds.map((id) => config.projects[id]);
  config.groups[payload.slug] = {
    id: payload.slug,
    label: payload.label || payload.slug.replaceAll('_', ' '),
    description: payload.description || '',
    components: componentIds,
    quick_links: deriveQuickLinks({ quick_links: normalizeUrlMap(payload.quick_links || {}) }, componentObjects.map(normalizeProject)),
  };
  const backup = await saveConfig(config);
  return { group: config.groups[payload.slug], backup };
}

async function readBody(req) {
  return new Promise((resolve, reject) => {
    let body = '';
    req.on('data', (chunk) => {
      body += chunk.toString();
      if (body.length > 1_000_000) {
        req.destroy();
        reject(new Error('Payload demasiado grande.'));
      }
    });
    req.on('end', () => {
      if (!body) return resolve({});
      try { resolve(JSON.parse(body)); } catch (error) { reject(error); }
    });
    req.on('error', reject);
  });
}

function sendJson(res, statusCode, payload) {
  const body = JSON.stringify(payload, null, 2);
  res.writeHead(statusCode, { 'content-type': 'application/json; charset=utf-8', 'cache-control': 'no-store' });
  res.end(body);
}

function sendText(res, statusCode, body) {
  res.writeHead(statusCode, { 'content-type': 'text/plain; charset=utf-8', 'cache-control': 'no-store' });
  res.end(body);
}

function contentType(filePath) {
  if (filePath.endsWith('.html')) return 'text/html; charset=utf-8';
  if (filePath.endsWith('.css')) return 'text/css; charset=utf-8';
  if (filePath.endsWith('.js')) return 'text/javascript; charset=utf-8';
  if (filePath.endsWith('.svg')) return 'image/svg+xml';
  return 'application/octet-stream';
}

async function staticRoot() {
  try { await stat(DIST_DIR); return DIST_DIR; } catch { return PUBLIC_DIR; }
}

async function serveStatic(req, res, pathname) {
  const base = await staticRoot();
  const requested = pathname === '/' ? '/index.html' : pathname;
  const filePath = path.normalize(path.join(base, requested));
  if (!filePath.startsWith(base)) {
    sendText(res, 403, 'Forbidden');
    return;
  }
  try {
    await stat(filePath);
    res.writeHead(200, { 'content-type': contentType(filePath) });
    createReadStream(filePath).pipe(res);
  } catch {
    // Let React own client-side routes.
    const indexPath = path.join(base, 'index.html');
    try {
      await stat(indexPath);
      res.writeHead(200, { 'content-type': 'text/html; charset=utf-8' });
      createReadStream(indexPath).pipe(res);
    } catch {
      sendText(res, 404, 'Not found');
    }
  }
}

async function findProject(projectId) {
  const config = await loadConfig();
  return config.projects[projectId] ? normalizeProject(config.projects[projectId]) : null;
}

async function handleApi(req, res, url) {
  if (req.method === 'GET' && url.pathname === '/api/projects') {
    const [docker, dashboard] = await Promise.all([getDockerSummary(), getDashboard()]);
    sendJson(res, 200, { generatedAt: new Date().toISOString(), root: ROOT, projectsFile: PROJECTS_FILE, docker, projects: dashboard.projects, groups: dashboard.groups });
    return;
  }

  if (req.method === 'POST' && url.pathname === '/api/detect') {
    const body = await readBody(req);
    sendJson(res, 200, await detectProject(body.path));
    return;
  }

  if (req.method === 'POST' && url.pathname === '/api/products') {
    const body = await readBody(req);
    sendJson(res, 201, await createProduct(body));
    return;
  }

  const groupActionMatch = url.pathname.match(/^\/api\/groups\/([^/]+)\/actions\/([^/]+)$/);
  if (req.method === 'POST' && groupActionMatch) {
    const [, groupId, action] = groupActionMatch;
    const config = await loadConfig();
    const group = config.groups[groupId];
    if (!group) return sendJson(res, 404, { error: 'Grupo no encontrado.' });
    const job = await createGroupJob(group, config.projects, action);
    sendJson(res, 202, { job });
    return;
  }

  const actionMatch = url.pathname.match(/^\/api\/projects\/([^/]+)\/actions\/([^/]+)$/);
  if (req.method === 'POST' && actionMatch) {
    const [, projectId, action] = actionMatch;
    const project = await findProject(projectId);
    if (!project) return sendJson(res, 404, { error: 'Proyecto no encontrado.' });
    const job = await createProjectJob(project, action);
    sendJson(res, 202, { job });
    return;
  }

  const logsMatch = url.pathname.match(/^\/api\/projects\/([^/]+)\/logs$/);
  if (req.method === 'GET' && logsMatch) {
    const [, projectId] = logsMatch;
    const project = await findProject(projectId);
    if (!project) return sendJson(res, 404, { error: 'Proyecto no encontrado.' });
    const logs = await getProjectLogs(project, url.searchParams.get('tail'));
    sendJson(res, logs.code === 0 ? 200 : 500, logs);
    return;
  }

  const jobMatch = url.pathname.match(/^\/api\/jobs\/([^/]+)$/);
  if (req.method === 'GET' && jobMatch) {
    const [, jobId] = jobMatch;
    const job = jobs.get(jobId);
    if (!job) return sendJson(res, 404, { error: 'Job no encontrado.' });
    sendJson(res, 200, { job });
    return;
  }

  sendJson(res, 404, { error: 'Endpoint no encontrado.' });
}

const server = http.createServer(async (req, res) => {
  const url = new URL(req.url || '/', `http://${req.headers.host || `${HOST}:${PORT}`}`);
  try {
    if (url.pathname.startsWith('/api/')) {
      await handleApi(req, res, url);
      return;
    }
    await serveStatic(req, res, url.pathname);
  } catch (error) {
    const statusCode = error.statusCode || 500;
    sendJson(res, statusCode, { error: error.message || 'Error inesperado.', stack: process.env.NODE_ENV === 'development' ? error.stack : undefined });
  }
});

server.listen(PORT, HOST, () => {
  console.log(`local-infra control panel listening on http://${HOST}:${PORT}`);
  console.log(`projects file: ${PROJECTS_FILE}`);
});
