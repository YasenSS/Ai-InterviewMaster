"use client";

import { useEffect, useState } from "react";

import { health, ready } from "@/shared/api/generated/InterviewMaster";

import { type ServiceState, summarizeStatus } from "./status";

const labels: Record<ServiceState, string> = {
  checking: "正在检查",
  ready: "全部就绪",
  partial: "依赖未就绪",
  offline: "API 未启动",
};

export function SystemStatus() {
  const [state, setState] = useState<ServiceState>("checking");
  const [version, setVersion] = useState("—");

  useEffect(() => {
    let active = true;
    async function check() {
      const [healthResult, readinessResult] = await Promise.allSettled([health(), ready()]);
      if (!active) return;
      const apiReady = healthResult.status === "fulfilled" && healthResult.value.status === "ok";
      const dependenciesReady =
        readinessResult.status === "fulfilled" && readinessResult.value.status === "ok";
      if (apiReady && healthResult.status === "fulfilled") {
        setVersion(healthResult.value.version);
      }
      setState(summarizeStatus(apiReady, dependenciesReady));
    }
    void check();
    return () => {
      active = false;
    };
  }, []);

  return (
    <div className="statusBody" data-state={state}>
      <div className="statusSummary">
        <span className="statusDot" aria-hidden="true" />
        <div>
          <strong>{labels[state]}</strong>
          <span>API / PostgreSQL / Redis</span>
        </div>
      </div>
      <dl>
        <div>
          <dt>API</dt>
          <dd>Go · go-zero</dd>
        </div>
        <div>
          <dt>VERSION</dt>
          <dd>{version}</dd>
        </div>
        <div>
          <dt>CONTRACT</dt>
          <dd>v1 · generated</dd>
        </div>
      </dl>
    </div>
  );
}
