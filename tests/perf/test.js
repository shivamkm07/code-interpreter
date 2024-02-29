import http from 'k6/http';
import exec from 'k6/execution';
import { textSummary } from 'https://jslib.k6.io/k6-summary/0.0.2/index.js';
import { jUnit } from 'https://jslib.k6.io/k6-summary/0.0.2/index.js';
import { check } from 'k6';
import { htmlReport } from "https://raw.githubusercontent.com/benc-uk/k6-reporter/main/dist/bundle.js";

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

function addTrendMetrics(metrics, prefix, values) {
  metrics.push([prefix + 'MIN', values.min.toFixed(2) + ' ms']);
  metrics.push([prefix + 'MAX', values.max.toFixed(2) + ' ms']);
  metrics.push([prefix + 'MED', values.med.toFixed(2) + ' ms']);
  metrics.push([prefix + 'AVG', values.avg.toFixed(2) + ' ms']);
  metrics.push([prefix + 'P95', values['p(95)'].toFixed(2) + ' ms']);
  metrics.push([prefix + 'P90', values['p(90)'].toFixed(2) + ' ms']);
}

function addCounterMetrics(metrics, prefix, values) {
  metrics.push([prefix + 'COUNT', values.count.toString()]);
  metrics.push([prefix + 'RATE', values.rate.toFixed(2)]);
}

function addRateMetrics(metrics, prefix, values) {
  metrics.push([prefix + 'PASSED', values.passes.toString()]);
  metrics.push([prefix + 'FAILED', values.fails.toString()]);
}

function addMaxGaugeMetrics(metrics, prefix, values) {
  metrics.push([prefix + 'MAX', values.max.toString()]);
}

function extractMetrics(data) {
    let metrics = [];
    addMaxGaugeMetrics(metrics, 'VUs ', data.metrics.vus.values);
    addCounterMetrics(metrics, 'Iterations ' ,data.metrics.iterations.values);
    addRateMetrics(metrics, 'Checks ', data.metrics.checks.values);
    addTrendMetrics(metrics, 'Req Duration ', data.metrics.http_req_duration.values);
    addTrendMetrics(metrics, 'Req Waiting ', data.metrics.http_req_waiting.values);
    return metrics;
}

export function handleSummary(data) {
    let metrics = extractMetrics(data);
    return {
      stdout: textSummary(data, {enableColors: true }),
      'test_perf_report_summary.json': JSON.stringify(data),
      'test_perf_report_summary.xml': jUnit(data),
      'test_perf_report_metrics.json': JSON.stringify(metrics),
      "test_perf_report_summary.html": htmlReport(data),
    };
}