*Message payload sent to Jupyter*

```json
{
  "buffers": [],
  "content": {
    "allow_stdin": false,
    "code": "printf(\"Hello Earth\")",
    "silent": false,
    "store_history": true,
    "user_expressions": {}
  },
  "header": {
    "msg_id": "dace3c80-8b15-49eb-9763-c379e23e1a76",
    "msg_type": "execute_request",
    "session": "523d3295-cf7d-48fb-beb0-7f387c112f20",
    "username": "username",
    "version": "5.3"
  },
  "metadata": {},
  "parent_header": {},
  "signature": "0620329d2b6739cc5a0ab877af69bfe8bc99739a8115b7e48b5d62700dacfc3f"
}
```

*GenericMessage from Jupyter converted to ExecutePlainResultTest*

```json
{
  "success": false,
  "errorCode": 0,
  "errorName": "NameError",
  "errorMessage": "name 'printf' is not defined",
  "errorTraceback": "\u001b[0;31m---------------------------------------------------------------------------\u001b[0m\n\u001b[0;31mNameError\u001b[0m                                 Traceback (most recent call last)\nCell \u001b[0;32mIn[949], line 1\u001b[0m\n\u001b[0;32m----> 1\u001b[0m \u001b[43mprintf\u001b[49m(\u001b[38;5;124m\"\u001b[39m\u001b[38;5;124mHello Earth\u001b[39m\u001b[38;5;124m\"\u001b[39m)\n\n\u001b[0;31mNameError\u001b[0m: name 'printf' is not defined\n",
  "executionDurationMilliseconds": 0
}
```

*ExecutePlainResultTest converted to ExecutionResponse and returned as response*

```json
{
  "hresult": -2147205116,
  "result": null,
  "error_name": "NameError",
  "error_message": "name 'printf' is not defined",
  "error_stack_trace": "\u001b[0;31m---------------------------------------------------------------------------\u001b[0m\n\u001b[0;31mNameError\u001b[0m                                 Traceback (most recent call last)\nCell \u001b[0;32mIn[949], line 1\u001b[0m\n\u001b[0;32m----> 1\u001b[0m \u001b[43mprintf\u001b[49m(\u001b[38;5;124m\"\u001b[39m\u001b[38;5;124mHello Earth\u001b[39m\u001b[38;5;124m\"\u001b[39m)\n\n\u001b[0;31mNameError\u001b[0m: name 'printf' is not defined\n",
  "stdout": "",
  "stderr": "",
  "diagnosticInfo": {
    "executionDuration": 183
  }
}
```