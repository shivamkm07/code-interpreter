import websocket
import json
import os
import threading
import uuid
import hmac
import hashlib

def create_header(msg_type):
    return {
        "msg_id": uuid.uuid4().hex,
        "username": "username",
        "session": uuid.uuid4().hex,
        "msg_type": msg_type,
        "version": "5.3",  # or other protocol version as needed
    }

def sign_message(header, parent_header, metadata, content, secret):
    h = hmac.new(secret.encode(), digestmod=hashlib.sha256)
    for part in [header, parent_header, metadata, content]:
        h.update(json.dumps(part).encode())
    return h.hexdigest()

def on_message(ws, message):
    print(f"Received message: {message}")
    try:
        msg = json.loads(message)
        header = msg["header"]
        msg_type = header["msg_type"]

        if msg_type == "stream":
            content = msg["content"]
            if content["name"] == "stdout":
                print("\n\nSTDOUT:", content["text"])
            elif content["name"] == "stderr":
                print("\n\nSTDERR:", content["text"])
        elif msg_type == "execute_result":
            content = msg["content"]
            print("\n\nRESULT:", content["data"])
    except Exception as e:
        print("Error processing message:", e)

def on_error(ws, error):
    print(error)

def on_close(ws, close_status_code, close_msg):
    print("### closed ###")

def on_open(ws):
    def run(*args):
        header = create_header("execute_request")
        parent_header = {}
        metadata = {}
        content = {
            "code": "print('Hello, Jupyter!')",
            "silent": False,
            "store_history": True,
            "user_expressions": {},
            "allow_stdin": False
        }
        secret = "your_secret_key"  # Replace with the actual key
        signature = sign_message(header, parent_header, metadata, content, secret)

        message = {
            "header": header,
            "parent_header": parent_header,
            "metadata": metadata,
            "content": content,
            "buffers": [],
            "signature": signature
        }

        ws.send(json.dumps(message))

    threading.Thread(target=run).start()

kernel_id = os.environ.get('KERNEL_ID')
token = os.environ.get('TOKEN')
if not kernel_id or not token:
    raise ValueError("KERNEL_ID and TOKEN environment variables are required")

ws_url = f"ws://localhost:8888/api/kernels/{kernel_id}/channels?token={token}"
websocket.enableTrace(True)
ws = websocket.WebSocketApp(ws_url,
                            on_open=on_open,
                            on_message=on_message,
                            on_error=on_error,
                            on_close=on_close)

ws.run_forever()