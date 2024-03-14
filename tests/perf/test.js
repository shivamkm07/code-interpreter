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
const ncusStageRegion = "northcentralusstage";

function getQPS(){
  if(__ENV.QPS){
    return parseInt(`${__ENV.QPS}`)
  }
  return 1;
}

function getDuration(){
  if(__ENV.DURATION){
    return `${__ENV.DURATION}`
  }
  return '10s';
}

function getScenarioType(){
  if(__ENV.SCENARIO_TYPE){
    return `${__ENV.SCENARIO_TYPE}`
  }
  return "constant";
}

let scenarios = {
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
  constant:{
    executor: 'constant-arrival-rate',
    rate: getQPS(),
    timeUnit: '1s',
    duration: getDuration(),
    preAllocatedVUs: 5*getQPS(),
  }
}

export const options = {
    // discardResponseBodies: true,
    thresholds: {
        checks: ['rate==1'],
    },
    scenarios: {
      scenario: scenarios[getScenarioType()],
    },
};

function getSessionID(){
  return getRunID() + `_${exec.scenario.iterationInTest}`;
}

function execute() {
    const url = 'http://localhost:8080/execute';
    const payload = JSON.stringify({
      code: "1+2",
      location: getRegion(),
    });
    let sessionId = getSessionID();
  
    const params = {
      headers: {
        'Content-Type': 'application/json',
        'IDENTIFIER': sessionId,
      },
      tags: {
        sessionId: sessionId,
      }
    };
  
    const res = http.post(url, payload, params);
    return res;
}

function recordXMsMetrics(headers, status){
    if('X-Ms-Allocation-Time' in headers){
      XMsAllocationTime.add(headers['X-Ms-Allocation-Time'], { sessionId: getSessionID(), status: status});
    }
    if('X-Ms-Container-Execution-Duration' in headers){
      XMsContainerExecutionDuration.add(headers['X-Ms-Container-Execution-Duration'], { sessionId: getSessionID(), status: status});
    }
    if('X-Ms-Execution-Read-Response-Time' in headers){
      XMsExecutionReadResponseTime.add(headers['X-Ms-Execution-Read-Response-Time'], { sessionId: getSessionID(), status: status});
    }
    if('X-Ms-Execution-Request-Time' in headers){
      XMsExecutionRequestTime.add(headers['X-Ms-Execution-Request-Time'], { sessionId: getSessionID(), status: status});
    }
    if('X-Ms-Overall-Execution-Time' in headers){
      XMsOverallExecutionTime.add(headers['X-Ms-Overall-Execution-Time'], { sessionId: getSessionID(), status: status});
    }
    if('X-Ms-Preparation-Time' in headers){
      XMsPreparationTime.add(headers['X-Ms-Preparation-Time'], { sessionId: getSessionID(), status: status});
    }
    if('X-Ms-Total-Execution-Service-Time' in headers){
      XMsTotalExecutionServiceTime.add(headers['X-Ms-Total-Execution-Service-Time'], { sessionId: getSessionID(), status: status});
    }
}

// function filterPointData(metric){
//   let data = JSON.parse(fs.readFileSync('./test_results.json', 'utf8'));
//   let filteredData = data.filter(item => item.type == "Point" && item.metric == metric);
//   return filteredData;
// }

export default function () {
    let result = execute();
    if(result.status < 200 || result.status >= 300){
      console.log("ERROR: Request failed with status: " + result.status + ". Response: " + result.body);
    }
    check(result, {
        'response code was 2xx': (result) =>
            result.status >= 200 && result.status < 300,
    })
    let status = result.status.toString();
    recordXMsMetrics(result.headers, status)
}

function addTrendMetrics(metrics, prefix, metric) {
  if(!(metric && metric.values)){
    return;
  }
  let values = metric.values;
  metrics.push([prefix + 'MIN', values.min]);
  metrics.push([prefix + 'MAX', values.max]);
  metrics.push([prefix + 'MED', values.med]);
  metrics.push([prefix + 'AVG', values.avg]);
  metrics.push([prefix + 'P90', values['p(90)']]);
  metrics.push([prefix + 'P95', values['p(95)']]);
  metrics.push([prefix + 'P98', values['p(98)']]);
  metrics.push([prefix + 'P99', values['p(99)']]);
  metrics.push([prefix + 'P99.9', values['p(99.9)']]);
}

function getRegion(){
  if(__ENV.REGION){
    return `${__ENV.REGION}`
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

function getRunID(){
  if(__ENV.RUN_ID){
    return `${__ENV.RUN_ID}`
  }
  return "Test";
}

function extractMetrics(data) {
    let metrics = [];
    let testStartTime = getTestStartTime();
    let testEndTime = new Date();
    let runID = getRunID();
    metrics.push(["RunID", runID]);
    metrics.push(["StartTime", testStartTime]);
    metrics.push(["EndTime", testEndTime]);
    metrics.push(["TestDuration_Min", ((testEndTime - testStartTime)/60000)]);
    metrics.push(["Region", getRegion()]);
    metrics.push(["RequestsTotal", data.metrics.iterations.values.count]);
    metrics.push(["RequestsPassed", data.metrics.checks.values.passes]);
    metrics.push(["RequestsFailed", data.metrics.checks.values.fails]);
    addTrendMetrics(metrics, "ReqDuration_Ms ", data.metrics.http_req_duration);
    addTrendMetrics(metrics,"XMsAllocationTime_Ms ",data.metrics.X_Ms_Allocation_Time);
    addTrendMetrics(metrics,"XMsContainerExecutionDuration_Ms ",data.metrics.X_Ms_Container_Execution_Duration);
    addTrendMetrics(metrics,"XMsExecutionReadResponseTime_Ms ",data.metrics.X_Ms_Execution_Read_Response_Time);
    addTrendMetrics(metrics,"XMsExecutionRequestTime_Ms ",data.metrics.X_Ms_Execution_Request_Time);
    addTrendMetrics(metrics,"XMsOverallExecutionTime_Ms ",data.metrics.X_Ms_Overall_Execution_Time);
    addTrendMetrics(metrics,"XMsPreparationTime_Ms ",data.metrics.X_Ms_Preparation_Time);
    addTrendMetrics(metrics,"XMsTotalExecutionServiceTime_Ms ",data.metrics.X_Ms_Total_Execution_Service_Time);

    return metrics;
}

function publishMetricsSummaryToEventHubs(metrics){
    let metricsObj = metrics.reduce((obj, item) => {
      obj[item[0]] = item[1];
      return obj;
    }, {});

    const url = 'http://localhost:8080/publish-metrics-summary';
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

function publishRealTimeMetricsToEventHubs(){
    const url = 'http://localhost:8080/publish-metrics-real-time';
    const params = {
      headers: {
        'RunID': getRunID(),
      },
    };

    const res = http.get(url, params);
    if(res.status < 200 || res.status >= 300){
      console.log("ERROR: PublishRealTimeMetricsToEventHubs Request failed with status: " + res.status + ". Response: " + res.body);
    } else{
      console.log("Real time metrics published to EventHubs successfully");
    }
    return res;
}

export function handleSummary(data) {
    let metrics = extractMetrics(data);
    publishMetricsSummaryToEventHubs(metrics);
    publishRealTimeMetricsToEventHubs();

    // Converting values to string as k6 summary does not support numbers
    metrics = metrics.map(item => [item[0], String(item[1])]);
    // httprePointMetrics = filterPointData("http_req_duration");
    // console.log("httprePointMetrics: " + JSON.stringify(httprePointMetrics));
    return {
      stdout: textSummary(data, {enableColors: true }),
      'test_perf_report_summary.json': JSON.stringify(data),
      'test_perf_report_summary.xml': jUnit(data),
      'test_perf_report_metrics.json': JSON.stringify(metrics),
      "test_perf_report_summary.html": htmlReport(data),
    };
}