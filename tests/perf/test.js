import http from 'k6/http';
import exec from 'k6/execution';
import { textSummary } from 'https://jslib.k6.io/k6-summary/0.0.2/index.js';
import { jUnit } from 'https://jslib.k6.io/k6-summary/0.0.2/index.js';
import { check } from 'k6';
import { htmlReport } from "https://raw.githubusercontent.com/benc-uk/k6-reporter/main/dist/bundle.js";
import { Trend } from "k6/metrics";

const XMsAllocationTime = new Trend('X_Ms_Allocation_Time');
const XMsContainerExecutionDuration = new Trend('X_Ms_Container_Execution_Duration');
const XMsExecutionReadResponseTime = new Trend('X_Ms_Execution_Read_Response_Time');
const XMsExecutionRequestTime = new Trend('X_Ms_Execution_Request_Time');
const XMsOverallExecutionTime = new Trend('X_Ms_Overall_Execution_Time');
const XMsPreparationTime = new Trend('X_Ms_Preparation_Time');
const XMsTotalExecutionServiceTime = new Trend('X_Ms_Total_Execution_Service_Time');

export const options = {
    discardResponseBodies: true,
    thresholds: {
        checks: ['rate==1'],
    },
    scenarios: {
        ramping: {
          executor: 'ramping-arrival-rate',
          startRate: 5,
          timeUnit: '1s',
          preAllocatedVUs: 20,
          stages: [
            { target: 5, duration: '1m' },
            { target: 10, duration: '1m' },
            { target: 10, duration: '1m' },
            { target: 15, duration: '1m' },
            { target: 15, duration: '1m' },
          ],
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
        // 'IDENTIFIER': `${exec.vu.idInTest}`,
      },
    };
  
    const res = http.post(url, payload, params);
    // console.log("response received: ", res)
    return res;
}

function recordXMsMetrics(headers){
    if('X-Ms-Allocation-Time' in headers){
      XMsAllocationTime.add(headers['X-Ms-Allocation-Time']);
    }
    if('X-Ms-Container-Execution-Duration' in headers){
      XMsContainerExecutionDuration.add(headers['X-Ms-Container-Execution-Duration']);
    }
    if('X-Ms-Execution-Read-Response-Time' in headers){
      XMsExecutionReadResponseTime.add(headers['X-Ms-Execution-Read-Response-Time']);
    }
    if('X-Ms-Execution-Request-Time' in headers){
      XMsExecutionRequestTime.add(headers['X-Ms-Execution-Request-Time']);
    }
    if('X-Ms-Overall-Execution-Time' in headers){
      XMsOverallExecutionTime.add(headers['X-Ms-Overall-Execution-Time']);
    }
    if('X-Ms-Preparation-Time' in headers){
      XMsPreparationTime.add(headers['X-Ms-Preparation-Time']);
    }
    if('X-Ms-Total-Execution-Service-Time' in headers){
      XMsTotalExecutionServiceTime.add(headers['X-Ms-Total-Execution-Service-Time']);
    }
}

export default function () {
    let result = execute();
    check(result, {
        'response code was 2xx': (result) =>
            result.status >= 200 && result.status < 300,
    })
    recordXMsMetrics(result.headers)
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
    addTrendMetrics(metrics,"X-Ms-Allocation-Time ",data.metrics.X_Ms_Allocation_Time.values);
    addTrendMetrics(metrics,"X-Ms-Container-Execution-Duration ",data.metrics.X_Ms_Container_Execution_Duration.values);
    addTrendMetrics(metrics,"X-Ms-Execution-Read-Response-Time ",data.metrics.X_Ms_Execution_Read_Response_Time.values);
    addTrendMetrics(metrics,"X-Ms-Execution-Request-Time ",data.metrics.X_Ms_Execution_Request_Time.values);
    addTrendMetrics(metrics,"X-Ms-Overall-Execution-Time ",data.metrics.X_Ms_Overall_Execution_Time.values);
    addTrendMetrics(metrics,"X-Ms-Preparation-Time ",data.metrics.X_Ms_Preparation_Time.values);
    addTrendMetrics(metrics,"X-Ms-Total-Execution-Service-Time ",data.metrics.X_Ms_Total_Execution_Service_Time.values);

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