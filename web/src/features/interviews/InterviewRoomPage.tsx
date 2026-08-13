"use client";

import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { AlertTriangle, ChevronLeft, Clock3, List, Save } from "lucide-react";
import Link from "next/link";
import { useEffect, useMemo, useState } from "react";

import { Alert, ErrorState } from "@/components/feedback/States";
import { Button } from "@/components/ui/Button";
import { Progress, Skeleton } from "@/components/ui/Display";
import { Textarea } from "@/components/ui/Form";
import { useAuth } from "@/features/auth/AuthGate";
import { api } from "@/shared/api/services";
import { normalizeError } from "@/shared/api/client";
import { queryKeys } from "@/shared/api/query";
import { draftKey, elapsedSeconds, questionRemaining } from "@/shared/lib/interview";
import { formatTimer } from "@/shared/lib/utils";
import { rememberInterview } from "@/shared/lib/recent";

export function InterviewRoomPage({ id }: { id: string }) {
  const { user } = useAuth();
  const queryClient = useQueryClient();
  const query = useQuery({ queryKey: queryKeys.interview(id), queryFn: () => api.interview(id) });
  const [now, setNow] = useState(0);
  const [enteredAt, setEnteredAt] = useState(0);
  const [answer, setAnswer] = useState("");
  const session = query.data;
  const current = session?.turns.find((turn) => turn.ordinal === session.current_ordinal) ?? session?.turns.find((turn) => !turn.answer) ?? session?.turns.at(-1);
  const key = user && current ? draftKey(user.id, id, current.ordinal) : "";

  useEffect(() => {
    const tick = window.setInterval(() => setNow(Date.now()), 1000);
    const sync = () => setNow(Date.now());
    const initial = window.setTimeout(sync, 0);
    document.addEventListener("visibilitychange", sync);
    return () => { window.clearTimeout(initial); window.clearInterval(tick); document.removeEventListener("visibilitychange", sync); };
  }, []);
  useEffect(() => {
    const restore = window.setTimeout(() => {
      setEnteredAt(Date.now());
      if (key) setAnswer(localStorage.getItem(key) ?? current?.answer ?? "");
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

  const mutation = useMutation({
    mutationFn: () => api.answerInterview(id, current!.ordinal, answer.trim()),
    onSuccess: (next) => {
      if (key) localStorage.removeItem(key);
      queryClient.setQueryData(queryKeys.interview(id), next);
      rememberInterview(next);
      setAnswer("");
    },
  });
  const remaining = now && enteredAt ? questionRemaining(enteredAt, now) : 180;
  const overallElapsed = session && now ? elapsedSeconds(session.created_at, now) : 0;
  const answered = session?.turns.filter((turn) => Boolean(turn.answer)).length ?? 0;
  const allAnswered = Boolean(session?.turns.length) && answered === (session?.turns.length ?? 0);
  const finished = session?.status === "completed" || allAnswered;
  const questionIndex = useMemo(() => session?.turns.findIndex((turn) => turn.ordinal === current?.ordinal) ?? 0, [current?.ordinal, session?.turns]);

  useEffect(() => {
    if (session?.status !== "active" || !allAnswered) return;
    void api.completeInterview(id).then((next) => {
      queryClient.setQueryData(queryKeys.interview(id), next);
      rememberInterview(next);
    }).catch(() => undefined);
  }, [allAnswered, id, queryClient, session?.status]);

  if (query.isPending) return <div className="interview-loading"><Skeleton className="skeleton-title" /><Skeleton className="skeleton-card tall" /></div>;
  if (query.error || !session || !current) return <div className="interview-loading"><ErrorState error={normalizeError(query.error)} retry={() => query.refetch()} /></div>;

  return (
    <div className="interview-room">
      <header className="interview-topbar">
        <Link href="/interviews"><ChevronLeft />退出</Link>
        <div className="timer-group"><span className={remaining <= 30 ? "timer-warning" : ""}><Clock3 />本题 {remaining ? formatTimer(remaining) : "已超时"}</span><span>整场 {formatTimer(overallElapsed)}</span></div>
        <span>第 {questionIndex + 1} / {session.turns.length} 题</span>
      </header>
      <Progress value={session.turns.length ? ((questionIndex + 1) / session.turns.length) * 100 : 0} />
      <main className="interview-content">
        <Alert title="按当前题目作答">保存后进入下一题。全部作答完成后会自动结束面试并开始生成报告。</Alert>
        <div className="question-prompt"><p className="eyebrow">{current.turn_kind === "follow_up" ? "追问" : "主问题"} · 第 {questionIndex + 1} 题</p><h1>{current.question}</h1><p className="answer-hint">建议结合具体情境、你的行动和可量化结果组织回答。追问计入本题时间，且不会替换剩余主问题。</p></div>
        {finished ? (
          <div className="interview-complete"><h2>本场面试已完成</h2><p>回答已锁定，可以查看结构化复盘报告。</p><Link className="button button-primary button-md" href={`/interviews/${id}/report`}>查看面试报告</Link></div>
        ) : (
          <div className="answer-panel"><label htmlFor="answer">你的回答</label><Textarea id="answer" value={answer} onChange={(event) => setAnswer(event.target.value)} placeholder="在这里组织你的回答…" className="answer-textarea" /><div className="draft-status"><Save size={15} />{answer && answer !== current.answer ? "草稿已保存在此设备" : "暂无未保存修改"}</div>{mutation.error ? <Alert title={normalizeError(mutation.error).message} tone="danger" /> : null}</div>
        )}
      </main>
      {!finished ? <footer className="interview-actions"><button disabled title="等待后端指定题接口"><ChevronLeft />上一题</button><button disabled title="等待后端跳题接口">跳过</button><Button onClick={() => mutation.mutate()} loading={mutation.isPending} disabled={!answer.trim()}>保存回答并继续</Button></footer> : null}
      <aside className="question-navigator" aria-label="题目导航"><h2><List />题目导航</h2>{session.turns.map((turn, index) => <button key={turn.ordinal} disabled className={turn.ordinal === current.ordinal ? "active" : ""}><span>{index + 1}</span>{turn.turn_kind === "follow_up" ? "追问" : turn.answer ? "已回答" : turn.ordinal === current.ordinal ? "作答中" : "未开始"}</button>)}<p><AlertTriangle size={15} />指定题跳转等待后端接口</p></aside>
    </div>
  );
}
