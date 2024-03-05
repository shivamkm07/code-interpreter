import http from 'k6/http';
import exec from 'k6/execution';
import { textSummary } from 'https://jslib.k6.io/k6-summary/0.0.2/index.js';
import { jUnit } from 'https://jslib.k6.io/k6-summary/0.0.2/index.js';
import { check, sleep } from 'k6';
import { htmlReport } from "https://raw.githubusercontent.com/benc-uk/k6-reporter/main/dist/bundle.js";
import { Trend } from "k6/metrics";

const XMsAllocationTime = new Trend('X_Ms_Allocation_Time');
const XMsContainerExecutionDuration = new Trend('X_Ms_Container_Execution_Duration');
const XMsExecutionReadResponseTime = new Trend('X_Ms_Execution_Read_Response_Time');
const XMsExecutionRequestTime = new Trend('X_Ms_Execution_Request_Time');
const XMsOverallExecutionTime = new Trend('X_Ms_Overall_Execution_Time');
const XMsPreparationTime = new Trend('X_Ms_Preparation_Time');
const XMsTotalExecutionServiceTime = new Trend('X_Ms_Total_Execution_Service_Time');
const ncusStageRegion = "North Central US(Stage)"

export const options = {
    // discardResponseBodies: true,
    thresholds: {
        checks: ['rate==1'],
    },
    scenarios: {
        test: {
          executor: 'shared-iterations',
          vus: 1,
          iterations: 10,
          maxDuration: '30s',
        },
        // ramping: {
        //   executor: 'ramping-arrival-rate',
        //   startRate: 5,
        //   timeUnit: '1s',
        //   preAllocatedVUs: 20,
        //   stages: [
        //     { target: 5, duration: '1m' },
        //     { target: 10, duration: '1m' },
        //     { target: 10, duration: '1m' },
        //     { target: 15, duration: '1m' },
        //     { target: 15, duration: '1m' },
        //   ],
        // },
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
        'IDENTIFIER': `${exec.scenario.iterationInTest}`,
      },
    };
  
    const res = http.post(url, payload, params);
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
    if(result.status < 200 || result.status >= 300){
      console.log("ERROR: Request failed with status: " + result.status + ". Response: " + result.body);
    }
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
  metrics.push([prefix + 'P90', values['p(90)'].toFixed(2) + ' ms']);
  metrics.push([prefix + 'P95', values['p(95)'].toFixed(2) + ' ms']);
  metrics.push([prefix + 'P98', values['p(98)'].toFixed(2) + ' ms']);
  metrics.push([prefix + 'P99', values['p(99)'].toFixed(2) + ' ms']);
  metrics.push([prefix + 'P99.9', values['p(99.9)'].toFixed(2) + ' ms']);
}

function getTestRegion(){
  if(__ENV.TEST_REGION){
    return `${__ENV.TEST_REGION}`
  }
    return ncusStageRegion;
}

function getTestStartTime(){
  if(__ENV.TEST_START_TIME){
    return new Date(`${__ENV.TEST_START_TIME}`);
  }
  console.log("INFO: Test start time not provided, using current time as start time");
  return new Date();
}

function extractMetrics(data) {
    let metrics = [];
    let testStartTime = getTestStartTime();
    let testEndTime = new Date();
    metrics.push(["StartTime", testStartTime.toISOString()]);
    metrics.push(["EndTime", testEndTime.toISOString()]);
    metrics.push(["TestDuration", String((testEndTime - testStartTime)/60000) + " min"]);
    metrics.push(["Region", getTestRegion()]);
    metrics.push(["RequestsTotal", String(data.metrics.iterations.values.count)]);
    metrics.push(["RequestsPassed", String(data.metrics.checks.values.passes)]);
    metrics.push(["RequestsFailed", String(data.metrics.checks.values.fails)]);
    addTrendMetrics(metrics, "ReqDuration ", data.metrics.http_req_duration.values);
    addTrendMetrics(metrics,"XMsAllocationTime ",data.metrics.X_Ms_Allocation_Time.values);
    addTrendMetrics(metrics,"XMsContainerExecutionDuration ",data.metrics.X_Ms_Container_Execution_Duration.values);
    addTrendMetrics(metrics,"XMsExecutionReadResponseTime ",data.metrics.X_Ms_Execution_Read_Response_Time.values);
    addTrendMetrics(metrics,"XMsExecutionRequestTime ",data.metrics.X_Ms_Execution_Request_Time.values);
    addTrendMetrics(metrics,"XMsOverallExecutionTime ",data.metrics.X_Ms_Overall_Execution_Time.values);
    addTrendMetrics(metrics,"XMsPreparationTime ",data.metrics.X_Ms_Preparation_Time.values);
    addTrendMetrics(metrics,"XMsTotalExecutionServiceTime ",data.metrics.X_Ms_Total_Execution_Service_Time.values);

    return metrics;
}

function publishMetricsToEventHubs(metrics){
    let metricsObj = metrics.reduce((obj, item) => {
      obj[item[0]] = item[1];
      return obj;
    }, {});

    const url = 'http://localhost:8080/publish-eventhubs';
    const payload = JSON.stringify(metricsObj);
    const params = {
      headers: {
        'Content-Type': 'application/json',
      },
    };

    const res = http.post(url, payload, params);
    if(res.status < 200 || res.status >= 300){
      console.log("ERROR: PublishMetricsToEventHubs Request failed with status: " + res.status + ". Response: " + res.body);
    } else{
      console.log("Metrics published to EventHubs successfully");
    }
    return res;
}

export function handleSummary(data) {
    let metrics = extractMetrics(data);
    publishMetricsToEventHubs(metrics);
    return {
      stdout: textSummary(data, {enableColors: true }),
      'test_perf_report_summary.json': JSON.stringify(data),
      'test_perf_report_summary.xml': jUnit(data),
      'test_perf_report_metrics.json': JSON.stringify(metrics),
      "test_perf_report_summary.html": htmlReport(data),
    };
}