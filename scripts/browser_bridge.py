"""Test transport for browser policies that disallow ALL URL navigation.
The browser renders the real compiled app in memory. Only this harness replaces
fetch/EventSource with a loopback HTTP bridge to the real test agent. This is NOT
native-network browser E2E or a validation of browser cookie/CSP enforcement.
No Chromium administrative policy is changed. No external URLs are fetched.
"""
import http.client, json, queue, threading, urllib.parse, asyncio
class LocalBridge:
    def __init__(self,url):
        parsed=urllib.parse.urlparse(url)
        assert parsed.hostname=='127.0.0.1'
        self.port=parsed.port;self.cookie='';self.streams={};self.counter=0
    def fetch(self,request):
        route=request['url'];assert route.startswith('/api/')
        headers=request.get('headers') or {}
        if self.cookie:headers['Cookie']=self.cookie
        conn=http.client.HTTPConnection('127.0.0.1',self.port,timeout=65)
        try:
            conn.request(request.get('method','GET'),route,body=request.get('body').encode('utf-8') if isinstance(request.get('body'),str) else request.get('body'),headers=headers)
            res=conn.getresponse();cookie=res.getheader('Set-Cookie')
            if cookie:self.cookie=cookie.split(';')[0]
            return {'status':res.status,'body':res.read().decode(),'headers':dict(res.getheaders())}
        finally:conn.close()
    def open(self,route):
        assert route.startswith('/api/events') or route.startswith('/api/logs?') or route.startswith('/api/infra/logs?')
        self.counter+=1;key=str(self.counter);q=queue.Queue(maxsize=1000);closed=threading.Event();state={'queue':q,'closed':closed,'conn':None};self.streams[key]=state
        cookie=self.cookie
        def reader():
            conn=http.client.HTTPConnection('127.0.0.1',self.port,timeout=35);state['conn']=conn
            try:
                conn.request('GET',route,headers={'Cookie':cookie});response=conn.getresponse()
                if response.status!=200:q.put_nowait({'type':'error'});return
                q.put_nowait({'type':'open'});event='message';data='';last=''
                while not closed.is_set():
                    line=response.readline().decode()
                    if not line:break
                    line=line.rstrip('\r\n')
                    if line.startswith('event: '):event=line[7:]
                    elif line.startswith('data: '):data=line[6:]
                    elif line.startswith('id: '):last=line[4:]
                    elif line=='' and data:
                        q.put_nowait({'type':event,'data':data,'lastEventId':last});data='';event='message'
            except Exception:
                if not closed.is_set():
                    try:q.put_nowait({'type':'error'})
                    except queue.Full:pass
            finally:conn.close()
        threading.Thread(target=reader,daemon=True).start();return key
    def poll(self,key):
        state=self.streams.get(key)
        if not state:return []
        result=[]
        for _ in range(100):
            try:result.append(state['queue'].get_nowait())
            except queue.Empty:break
        return result
    def close(self,key):
        state=self.streams.pop(key,None)
        if state:
            state['closed'].set()
            try:
                if state['conn'] and state['conn'].sock:
                    import socket
                    state['conn'].sock.shutdown(socket.SHUT_RDWR)
                if state['conn']:state['conn'].close()
            except OSError:pass
    def attach(self,page):
        async def fetch_async(request):
            return await asyncio.to_thread(self.fetch,request)
        page.expose_function('__nearprodHttp',fetch_async)
        page.expose_function('__nearprodStreamOpen',self.open)
        page.expose_function('__nearprodStreamPoll',self.poll)
        page.expose_function('__nearprodStreamClose',self.close)
        page.add_script_tag(content=r'''
window.fetch = async function(url,options={}) {
 if(options.signal?.aborted) throw new DOMException('Aborted','AbortError');
 const value=await window.__nearprodHttp({url:String(url),method:options.method||'GET',headers:options.headers||{},body:options.body});
 return new Response(value.body,{status:value.status,headers:value.headers});
};
window.EventSource=class extends EventTarget {
 constructor(url){super();this.closed=false;this.key=null;this.timer=null;this.onmessage=null;this.onerror=null;window.__nearprodStreamOpen(String(url)).then(key=>{this.key=key;if(this.closed){window.__nearprodStreamClose(key);return;}this.timer=setInterval(async()=>{if(this.closed)return;const events=await window.__nearprodStreamPoll(key);for(const v of events){const e=new MessageEvent(v.type,{data:v.data,lastEventId:v.lastEventId||''});this.dispatchEvent(e);if(v.type==='message')this.onmessage?.(e);if(v.type==='error')this.onerror?.(e);}},100);});}
 close(){this.closed=true;clearInterval(this.timer);if(this.key)window.__nearprodStreamClose(this.key);}
};
''')
