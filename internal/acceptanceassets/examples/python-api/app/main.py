"""Small ASGI app, without a framework dependency. Uvicorn handles HTTP and reload."""
import json
MESSAGE = "nearprod-python-v1"
async def app(scope, receive, send):
    if scope["type"] != "http":
        return
    body = json.dumps({"message": MESSAGE, "path": scope["path"]}).encode()
    await send({"type": "http.response.start", "status": 200,
                "headers": [(b"content-type", b"application/json")]})
    await send({"type": "http.response.body", "body": body})
