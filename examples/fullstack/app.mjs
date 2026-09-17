/** Small, dependency-free frontend/API acceptance fixture. Not a production framework. */
import http from 'node:http';
const role=process.env.ROLE || 'api', port=Number(process.env.PORT || 3000);
const apiURL=new URL(process.env.API_URL || 'http://api-demo.localhost');
const frontendURL=new URL(process.env.FRONTEND_URL || 'http://demo.localhost');
if(!['http:','https:'].includes(apiURL.protocol)) throw new Error('API_URL must be HTTP(S)');
const html=`<!doctype html><html lang="es"><meta charset="utf-8"><meta name="viewport" content="width=device-width"><title>NearProd · Frontend de prueba</title><style>body{font:18px system-ui;max-width:760px;margin:10vh auto;padding:20px}button{font:inherit;padding:12px 20px}pre{white-space:pre-wrap}</style><h1>Frontend de prueba</h1><p>Esta página se sirve desde una imagen. El botón consulta la API por su dominio.</p><button id="fetch">Consultar API</button><pre id="result">Listo.</pre><script type="module">document.querySelector('#fetch').onclick=async()=>{const out=document.querySelector('#result');out.textContent='Consultando…';try{const r=await fetch(${JSON.stringify(apiURL.origin + '/api/message').replace(/</g,'\\u003c')});if(!r.ok)throw Error('HTTP '+r.status);out.textContent=JSON.stringify(await r.json(),null,2)}catch(e){out.textContent='Error: '+e.message+' — comprueba dominios, CORS y que la API esté iniciada.'}};</script></html>`;
http.createServer((req,res)=>{
  console.log(`${role} ${req.method} ${req.url}`);
  if(req.url==='/health'){res.writeHead(200,{'content-type':'application/json'});return res.end('{"status":"ok"}');}
  if(role==='frontend'&&req.url==='/'){res.writeHead(200,{'content-type':'text/html; charset=utf-8'});return res.end(html);}
  if(role==='api'){
    if(req.headers.origin===frontendURL.origin)res.setHeader('access-control-allow-origin',frontendURL.origin);
    res.setHeader('vary','Origin');
    if(req.method==='OPTIONS'){res.writeHead(204,{'access-control-allow-methods':'GET,OPTIONS'});return res.end();}
    if(req.url==='/api/message'||req.url==='/'){res.writeHead(200,{'content-type':'application/json'});return res.end(JSON.stringify({message:'nearprod-api',language:'JavaScript',frontend:frontendURL.origin}));}
  }
  res.writeHead(404,{'content-type':'application/json'});res.end('{"error":"not_found"}');
}).listen(port,'0.0.0.0',()=>console.log(`${role} listening on 0.0.0.0:${port}`));
