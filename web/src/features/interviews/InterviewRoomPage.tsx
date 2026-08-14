"use client";

import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { ChevronLeft, Clock3, History, Save, Sparkles } from "lucide-react";
import Link from "next/link";
import { useRouter } from "next/navigation";
import { useEffect, useMemo, useState } from "react";

import { Alert, ErrorState } from "@/components/feedback/States";
import { Button } from "@/components/ui/Button";
import { Card, Skeleton } from "@/components/ui/Display";
import { Textarea } from "@/components/ui/Form";
import { useAuth } from "@/features/auth/AuthGate";
import { normalizeError } from "@/shared/api/client";
import { cacheTimes, queryKeys } from "@/shared/api/query";
import { api } from "@/shared/api/services";
import { draftKey, elapsedSeconds } from "@/shared/lib/interview";
import { rememberInterview } from "@/shared/lib/recent";
import { formatTimer } from "@/shared/lib/utils";

const preparingStatuses = new Set(["pending", "preparing", "queued", "generating"]);

export function InterviewRoomPage({ id }: { id: string }) {
  const { user } = useAuth();
  const router = useRouter();
  const queryClient = useQueryClient();
  const query = useQuery({
    queryKey: queryKeys.interview(id),
    queryFn: () => api.interview(id),
    refetchInterval: (result) => {
      if (document.visibilityState !== "visible") return false;
      const session = result.state.data;
      if (!session) return cacheTimes.taskPoll;
      const hasOpenTurn = session.turns.some((turn) => !turn.answer && turn.state !== "answered" && turn.state !== "skipped");
      return preparingStatuses.has(session.status) || (session.status === "active" && !hasOpenTurn)
        ? cacheTimes.taskPoll
        : false;
    },
  });
  const [now, setNow] = useState(0);
  const [enteredAt, setEnteredAt] = useState(0);
  const [answer, setAnswer] = useState("");
  const session = query.data;

  const visibleTurns = useMemo(() => {
    if (!session) return [];
    return session.turns.filter((turn) =>
      Boolean(turn.question) && (
        turn.ordinal <= session.current_ordinal ||
        Boolean(turn.answer) ||
        turn.state === "answered" ||
        turn.state === "skipped"
      ),
    );
  }, [session]);
  const current = visibleTurns.find((turn) =>
    turn.ordinal === session?.current_ordinal && !turn.answer && turn.state !== "answered" && turn.state !== "skipped",
  ) ?? visibleTurns.find((turn) => !turn.answer && turn.state !== "answered" && turn.state !== "skipped");
  const key = user && current ? draftKey(user.id, id, current.ordinal) : "";

  useEffect(() => {
    const tick = window.setInterval(() => setNow(Date.now()), 1000);
    const sync = () => setNow(Date.now());
    const initial = window.setTimeout(sync, 0);
    document.addEventListener("visibilitychange", sync);
    return () => {
      window.clearTimeout(initial);
      window.clearInterval(tick);
      document.removeEventListener("visibilitychange", sync);
    };
  }, []);

  useEffect(() => {
    const restore = window.setTimeout(() => {
      setEnteredAt(Date.now());
      setAnswer(key ? localStorage.getItem(key) ?? current?.answer ?? "" : "");
    }, 0);
    return () => window.clearTimeout(restore);
  }, [current?.answer, current?.ordinal, key]);

  useEffect(() => {
    if (!key) return;
    const timer = window.setTimeout(() => {
      if (answer.trim() && answer !== current?.answer) localStorage.setItem(key, answer);
    }, 300);
    return () => window.clearTimeout(timer);
  }, [answer, current?.answer, key]);

  useEffect(() => {
    const protect = (event: BeforeUnloadEvent) => {
      if (answer.trim() && answer !== current?.answer) event.preventDefault();
    };
    window.addEventListener("beforeunload", protect);
    return () => window.removeEventListener("beforeunload", protect);
  }, [answer, current?.answer]);

  useEffect(() => {
    if (session?.status === "completed") router.replace(`/interviews/${id}/record`);
  }, [id, router, session?.status]);

  const mutation = useMutation({
    mutationFn: () => api.answerInterview(id, current!.ordinal, answer.trim()),
    onSuccess: (next) => {
      if (key) localStorage.removeItem(key);
      queryClient.setQueryData(queryKeys.interview(id), next);
      rememberInterview(next);
      setAnswer("");
      if (next.status === "completed") {
        router.replace(`/interviews/${id}/record`);
      } else {
        void query.refetch();
      }
    },
  });
  const retryPreparation = useMutation({
    mutationFn: () => api.retryTask(session!.task_id!),
    onSuccess: async () => {
      await query.refetch();
      void queryClient.invalidateQueries({ queryKey: queryKeys.interviews() });
    },
  });

  if (query.isPending) {
    return <div className="interview-loading"><Skeleton className="skeleton-title" /><Skeleton className="skeleton-card tall" /></div>;
  }
  if (query.error || !session) {
    return <div className="interview-loading"><ErrorState error={normalizeError(query.error)} retry={() => query.refetch()} /></div>;
  }
  if (session.status === "failed") {
    return (
      <div className="page narrow-page">
        <Card className="processing-state">
          <h1>本次面试准备失败</h1>
          <p>简历和历史记录没有受到影响，可以直接重试本次准备，也可以重新选择配置。</p>
          {retryPreparation.error ? <Alert title={normalizeError(retryPreparation.error).message} tone="danger" /> : null}
          <div className="button-row">
            {session.task_id ? <Button onClick={() => retryPreparation.mutate()} loading={retryPreparation.isPending}>重试本次准备</Button> : null}
            <Link className="button button-secondary button-md" href="/interviews/new">重新配置</Link>
          </div>
        </Card>
      </div>
    );
  }
  if (preparingStatuses.has(session.status) || (session.status === "active" && !current)) {
    return (
      <div className="page narrow-page">
        <Card className="processing-state interview-preparing">
          <span className="processing-orb" />
          <h1>{visibleTurns.length ? "正在决定下一问" : "正在准备面试"}</h1>
          <p>{visibleTurns.length ? "模型正在结合刚才的回答选择追问或切换考察方向。" : "正在结合简历、技术语言和目标公司准备第一个问题。"}</p>
          <div className="preparing-steps" aria-label="准备步骤">
            <span className="done">读取简历</span><span className="active">匹配考察方向</span><span>进入面试</span>
          </div>
          <Link className="back-link" href="/interviews/records">稍后从面试记录返回</Link>
        </Card>
      </div>
    );
  }
  if (session.status === "completed") {
    return <div className="interview-loading"><Skeleton className="skeleton-card tall" /></div>;
  }
  if (!current) {
    return <div className="interview-loading"><ErrorState error={{ code: "TURN_NOT_READY", message: "当前问题暂未准备好，请稍后重试。" }} retry={() => query.refetch()} /></div>;
  }

  const duration = session.question_duration_seconds || 180;
  const remaining = now && enteredAt ? Math.max(0, duration - Math.floor((now - enteredAt) / 1000)) : duration;
  const overallElapsed = now ? elapsedSeconds(session.started_at ?? session.created_at, now) : 0;
  const answeredCount = visibleTurns.filter((turn) => Boolean(turn.answer) || turn.state === "answered").length;

  return (
    <div className="interview-room dynamic-interview-room">
      <header className="interview-topbar">
        <Link href="/interviews/records"><ChevronLeft />退出</Link>
        <div className="timer-group">
          <span className={remaining <= 30 ? "timer-warning" : ""}><Clock3 />本轮 {remaining ? formatTimer(remaining) : "已超时"}</span>
          <span>整场 {formatTimer(overallElapsed)}</span>
        </div>
        <span>第 {visibleTurns.length} 轮</span>
      </header>
      <main className="interview-content">
        <div className="question-prompt">
          <p className="eyebrow"><Sparkles size={15} />{current.turn_kind === "follow_up" ? "基于上一轮的追问" : "面试官提问"} · 第 {visibleTurns.length} 轮</p>
          <h1>{current.question}</h1>
          <p className="answer-hint">结合具体情境、你的行动和可量化结果回答。提交后，面试官会根据回答决定下一步。</p>
        </div>
        <div className="answer-panel">
          <label htmlFor="answer">你的回答</label>
          <Textarea id="answer" value={answer} onChange={(event) => setAnswer(event.target.value)} placeholder="在这里组织你的回答…" className="answer-textarea" autoFocus />
          <div className="draft-status"><Save size={15} />{answer && answer !== current.answer ? "草稿已保存在此设备" : "暂无未保存修改"}</div>
          {mutation.error ? <Alert title={normalizeError(mutation.error).message} tone="danger" /> : null}
        </div>
      </main>
      <footer className="interview-actions dynamic-interview-actions">
        <span className="interview-turn-count"><History size={16} />已完成 {answeredCount} 轮</span>
        <Button onClick={() => mutation.mutate()} loading={mutation.isPending} disabled={!answer.trim()}>
          提交回答
        </Button>
      </footer>
    </div>
  );
}
