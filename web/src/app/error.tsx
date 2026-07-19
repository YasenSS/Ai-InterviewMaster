"use client";

export default function ErrorPage({ reset }: { reset: () => void }) {
  return (
    <div className="routeMessage">
      <p>页面暂时无法载入。</p>
      <button type="button" onClick={reset}>
        重试
      </button>
    </div>
  );
}
