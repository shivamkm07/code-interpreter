import http from 'k6/http';
import exec from 'k6/execution';
import { textSummary } from 'https://jslib.k6.io/k6-summary/0.0.2/index.js';
import { check } from 'k6';
export const options = {
    discardResponseBodies: true,
    thresholds: {
        checks: ['rate==1'],
    },
    scenarios: {
        'base': {
            executor: 'shared-iterations',
            vus: 1,
            iterations: 10,
            maxDuration: '10s',
        },
    },
};

function execute() {
    const url = 'http://localhost:8080/execute';
    const payload = JSON.stringify({
      code: "1+2"
    });
  
    const params = {
      headers: {
        'Content-Type': 'application/json',
        'IDENTIFIER': `${exec.vu.idInTest}`,
      },
    };
  
    const res = http.post(url, payload, params);
    return res;
}

export default function () {
    let result = execute();
    check(result, {
        'response code was 2xx': (result) =>
            result.status >= 200 && result.status < 300,
    })
}

export function handleSummary(data) {
    // for (const key in data.metrics) {
    //   if (key.startsWith('data_')) delete data.metrics[key];
    // }
  
    return {
      stdout: textSummary(data, {enableColors: true }),
    };
}