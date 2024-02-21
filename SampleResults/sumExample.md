*Message payload sent to Jupyter*

```json
{
    "buffers": [],
    "content": {
        "allow_stdin": false,
        "code": "1+1",
        "silent": false,
        "store_history": true,
        "user_expressions": {}
    },
    "header": {
        "msg_id": "24f8613e-789c-4e54-b4fb-1d813b2a598e",
        "msg_type": "execute_request",
        "session": "96068463-834b-4cad-8c42-e70faa064ac8",
        "username": "username",
        "version": "5.3"
    },
    "metadata": {},
    "parent_header": {},
    "signature": "307001c78defb3135583a84aeabb3720ddc2b9b9b345e50798b7b730700ca02c"
}
```

*GenericMessage from Jupyter converted to ExecutePlainResultTest*

```json
{
    "success": true,
    "errorCode": 0,
    "textPlain": "2",
    "executionDurationMilliseconds": 0
}
```

*ExecutePlainResultTest converted to ExecutionResponse and returned as response*

```json
{
    "hresult": 0,
    "result": 33,
    "error_name": "",
    "error_message": "",
    "error_stack_trace": "",
    "stdout": "",
    "stderr": "",
    "diagnosticInfo": {
      "executionDuration": 13
    }
}
```