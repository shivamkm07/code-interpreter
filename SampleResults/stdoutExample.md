*Message payload sent to Jupyter*

```json
{
    "buffers": [],
    "content": {
        "allow_stdin": false,
        "code": "print(\"Hello Earth\")",
        "silent": false,
        "store_history": true,
        "user_expressions": {}
    },
    "header": {
        "msg_id": "ec5c3938-c23b-4788-b2e7-73e3da604447",
        "msg_type": "execute_request",
        "session": "d7c14f99-e7c8-4a3b-ab79-5dc316e777a2",
        "username": "username",
        "version": "5.3"
    },
    "metadata": {},
    "parent_header": {},
    "signature": "8a8860de1b7e76b8204c278c2502024e5ac6613d4fb62d4cd29afa65f155dea6"
}
```

*GenericMessage from Jupyter converted to ExecutePlainResultTest*

```json
{
    "success": true,
    "errorCode": 0,
    "stdout": "Hello Earth\n",
    "executionDurationMilliseconds": 0
}
```

*ExecutePlainResultTest converted to ExecutionResponse and returned as response*

```json
{
    "hresult": 0,
    "result": "",
    "error_name": "",
    "error_message": "",
    "error_stack_trace": "",
    "stdout": "Hello Earth\n",
    "stderr": "",
    "diagnosticInfo": {
        "executionDuration": 17
    }
}
```