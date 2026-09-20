"""Native installed CLI/HTTP/migration tests, real filesystem; no Docker needed.
Runs on the host platform against a prebuilt binary, never a Node wrapper.
"""
import gzip,hashlib,json,os,platform,select,subprocess,sys,tarfile,tempfile,time,urllib.request,shutil
from pathlib import Path
R=Path(__file__).resolve().parents[1];E=R/'evidence';E.mkdir(exist_ok=True)
arch={'x86_64':'amd64','aarch64':'arm64','arm64':'arm64'}[platform.machine()];system=platform.system().lower()
B=Path(sys.argv[1]).resolve() if len(sys.argv)>1 else R/'bin'/f'nearprod-{system}-{arch}'
checks=[];report={'platform':platform.platform(),'binarySha256':hashlib.sha256(B.read_bytes()).hexdigest(),'checks':checks,'state':'running','docker':'not used'}
def ok(s):checks.append(s);print('PASS',s,flush=True)
with tempfile.TemporaryDirectory(prefix='np-native-install-',dir='/tmp') as temp:
 h=Path(temp);home=h/'.nearprod';home.mkdir();(h/'.zshrc').write_text('# configuración del usuario\nexport TEST_VALUE=kept\n')
 d=json.loads((R/'tests/fixtures/catalog-0.6.1.json').read_text());legacy=json.dumps(d,ensure_ascii=False,indent=2).encode();(home/'catalog.json').write_bytes(legacy);(home/'catalog.json').chmod(0o600)
 # synthetic legacy vault; never real application credentials
 r=d['infra']['instances'][0];vpath=home/'infra'/r['uid']/'vault.json';vpath.parent.mkdir(parents=True);vault={'owner':d['owner'],'uid':r['uid'],'admin':'fixture-admin','databases':{d['infra']['databases'][0]['id']:{'password':'fixture-password'}}};vpath.write_text(json.dumps(vault));vpath.chmod(0o600)
 env={**os.environ,'HOME':str(h),'NEARPROD_HOME':str(home),'PATH':'/usr/bin:/bin','XDG_CONFIG_HOME':str(h/'.config')}
 def run(*a,binary=B,check=True):
  p=subprocess.run([str(binary),*a],env=env,text=True,capture_output=True,timeout=20)
  if check and p.returncode:raise AssertionError(p.stderr+p.stdout)
  return p
 installed=h/'.local/bin/nearprod'
 try:
  expected_version=sys.argv[2] if len(sys.argv)>2 else (R/'VERSION').read_text().strip()
  assert run('--version').stdout.strip()==expected_version;ok('Binario nativo sin Node en PATH')
  data=json.loads(run('install','--configure-shell','--json').stdout);assert installed.is_file();ok('Instalación nativa atómica real')
  assert (home/'catalog.json').read_bytes()==legacy;ok('Instalar no modifica ni migra el catálogo')
  assert (h/'.zshrc').read_text().count('# NearProd: comando estable')==1;assert len(list(h.glob('.zshrc.nearprod-*.bak')))==1;ok('Shell conservado con backup privado')
  data=json.loads(run('config','migrate','--dry-run','--json',binary=installed).stdout);assert data['requiresMigration'];assert not (home/'config/catalog.json').exists();ok('Vista previa no escribe ni ejecuta Docker')
  run('config','migrate','--yes','--json',binary=installed);after=json.loads((home/'config/catalog.json').read_text());assert after['version']==5
  for k in ('owner','roots','groups','stacks','runtime','proxy'):assert after[k]==d[k],k
  for k in ('instances','databases','bindings'):assert after['infra'][k]==d['infra'][k],k
  assert after['archivedStacks']==[]
  assert after['infra']['archivedInstances']==[] and after['infra']['archivedDatabases']==[]
  ok('Migración preserva proyectos URLs credenciales referencias y grupos')
  assert json.loads((home/'catalog.json').read_text())['version']==-1;assert json.loads(vpath.read_text())==vault;ok('Writer antiguo bloqueado sin reubicar vault ni volumen')
  journal=json.loads((home/'config/migration.json').read_text());assert Path(journal['backup']).read_bytes()==legacy;ok('Backup original byte por byte y journal finalizado')
  data=json.loads(run('ui','--no-open','--json',binary=installed).stdout);info=json.loads((home/'agent.json').read_text());ok('Agente nativo real y bootstrap local')
  def get(path,auth=False):
   req=urllib.request.Request(info['url']+path,headers={'Authorization':'Bearer '+info['token']} if auth else {});return urllib.request.urlopen(req,timeout=5)
  with get('/') as response:assert b'<div id="root">' in response.read()
  for name in ['/vendor/react.js','/ui/App.js','/ui/infrastructure.js','/ui/McpAccessView.js','/styles.css']:
   with get(name) as response:assert len(response.read())>50
  with get('/api/mcp',True) as response:mcp=json.loads(response.read())
  assert mcp['transport']=='stdio' and mcp['args']==['mcp','serve'] and info['token'] not in json.dumps(mcp)
  ok('Assets React/TypeScript servidos por HTTP del binario')
  initialize={'jsonrpc':'2.0','id':1,'method':'initialize','params':{'protocolVersion':'2025-06-18','capabilities':{},'clientInfo':{'name':'package-test','version':'1'}}}
  mcp_process=subprocess.Popen([str(installed),'mcp','serve'],env=env,stdin=subprocess.PIPE,stdout=subprocess.PIPE,stderr=subprocess.PIPE,text=True)
  mcp_process.stdin.write(json.dumps(initialize)+'\n');mcp_process.stdin.flush();ready,_,_=select.select([mcp_process.stdout],[],[],10);assert ready
  mcp_response=json.loads(mcp_process.stdout.readline());mcp_process.stdin.close();mcp_process.wait(timeout=10);mcp_error=mcp_process.stderr.read()
  assert mcp_process.returncode==0 and not mcp_error and mcp_response['result']['serverInfo']['name']=='nearprod'
  ok('Servidor MCP stdio responde sin contaminar stdout')
  c=json.loads(run('list','--json',binary=installed).stdout);assert len(c['stacks'])==1 and c['groups'][0]['name']=='Máximo Puntaje';ok('CLI y HTTP conservan catálogo migrado sin Docker')
  run('group-create','--name','Temporal','--id','temporal','--json',binary=installed);run('group-delete','temporal','--yes','--json',binary=installed);assert all(g['id']!='temporal' for g in json.loads(run('groups','--json',binary=installed).stdout)['groups']);ok('CLI elimina únicamente grupo vacío con preview firmado')
  run('init',str(h),'--json',binary=installed);run('root-remove',str(h),'--yes','--json',binary=installed);assert str(h) not in json.loads(run('list','--json',binary=installed).stdout)['roots'];assert h.is_dir();ok('CLI retira raíz sin dependencias y conserva filesystem')
  data=json.loads(run('config','backup','--yes','--json',binary=installed).stdout);file=Path(data.get('file') or data.get('path') or data.get('backup') or '')
  if not file.is_file(): raise AssertionError(data)
  assert file.stat().st_mode & 0o777==0o600
  with tarfile.open(file) as tf:
   names=tf.getnames();assert 'config/catalog.json' in names;assert any(n.endswith('vault.json') for n in names);assert not any(n.startswith('databases/') for n in names)
  ok('Backup de configuración incluye vault sin copiar datos físicos')
  run('agent','stop',binary=installed);ok('Parada nativa sin tocar Docker')
  run('install','--configure-shell',binary=B);assert (h/'.zshrc').read_text().count('# NearProd: comando estable')==1;ok('Reinstalación conserva catálogo y configuración shell única')
  run('ui','--no-open','--json',binary=installed);c=json.loads(run('list','--json',binary=installed).stdout);assert c['stacks'][0]['uid']==d['stacks'][0]['uid'];ok('Reinicio/reinstalación sin registrar nuevamente')
  run('agent','stop',binary=installed)
  foreign=installed.with_suffix('.replacement');foreign.write_text('#!/bin/sh\necho foreign\n');foreign.chmod(0o755);os.replace(foreign,installed)
  res=run('install','--configure-shell',binary=B,check=False);assert res.returncode and 'INSTALL_FOREIGN' in res.stderr;ok('Instalador rechaza reemplazar un comando ajeno')
  report['state']='passed'
 except Exception as e:
  report['state']='failed';report['error']=str(e);raise
 finally:
  if installed.exists():
   try:run('agent','stop',binary=B,check=False)
   except Exception:pass
  (E/'native-package.json').write_text(json.dumps(report,indent=2,ensure_ascii=False))
