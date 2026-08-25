"use client";

import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { Bot, ChevronLeft, Clock3, Send, UserRound } from "lucide-react";
import Link from "next/link";
import { useRouter } from "next/navigation";
import { useEffect, useMemo, useRef, useState, type KeyboardEvent } from "react";

import { Alert, ErrorState } from "@/components/feedback/States";
import { Button } from "@/components/ui/Button";
import { Badge, Card, Skeleton } from "@/components/ui/Display";
import { Textarea } from "@/components/ui/Form";
import { useAuth } from "@/features/auth/AuthGate";
import { normalizeError } from "@/shared/api/client";
import { cacheTimes, queryKeys } from "@/shared/api/query";
import { api } from "@/shared/api/services";
import type { InterviewTurnResponse } from "@/shared/api/generated/InterviewMasterComponents";
import { draftKey, elapsedSeconds, isFollowUpTurn, shouldCollapseChatText, visibleInterviewTurns } from "@/shared/lib/interview";
import { rememberInterview } from "@/shared/lib/recent";
import { formatTimer } from "@/shared/lib/utils";

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
      return session.status === "preparing" || session.phase === "deciding"
        ? cacheTimes.taskPoll
        : false;
    },
  });
  const [now, setNow] = useState(0);
  const [enteredAt, setEnteredAt] = useState(0);
  const [answer, setAnswer] = useState("");
  const threadRef = useRef<HTMLDivElement>(null);
  const composerRef = useRef<HTMLTextAreaElement>(null);
  const stickToBottomRef = useRef(true);
  const session = query.data;

  const visibleTurns = useMemo(
    () => (session ? visibleInterviewTurns(session.turns, session.current_ordinal) : []),
    [session],
  );
  const current = session?.phase === "answering"
    ? visibleTurns.find((turn) => turn.ordinal === session.current_ordinal && !turn.answer && turn.state !== "skipped")
    : undefined;
  const key = user && current ? draftKey(user.id, id, current.ordinal) : "";
  const answering = session?.phase === "answering" && Boolean(current);
  const deciding = session?.phase === "deciding";
  const decisionFailed = session?.phase === "decision_failed";

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
      setAnswer(key ? localStorage.getItem(key) ?? "" : "");
    }, 0);
    return () => window.clearTimeout(restore);
  }, [current?.ordinal, key]);

  useEffect(() => {
    if (!key) return;
    const timer = window.setTimeout(() => {
      if (answer.trim()) localStorage.setItem(key, answer);
    }, 300);
    return () => window.clearTimeout(timer);
  }, [answer, key]);

  useEffect(() => {
    const protect = (event: BeforeUnloadEvent) => {
      if (answer.trim()) event.preventDefault();
    };
    window.addEventListener("beforeunload", protect);
    return () => window.removeEventListener("beforeunload", protect);
  }, [answer]);

  useEffect(() => {
    if (session?.status === "completed") router.replace(`/interviews/${id}/record`);
  }, [id, router, session?.status]);

  const acceptResponse = (next: NonNullable<typeof session>) => {
    if (key) localStorage.removeItem(key);
    queryClient.setQueryData(queryKeys.interview(id), next);
    rememberInterview(next);
    setAnswer("");
    if (next.status === "completed") router.replace(`/interviews/${id}/record`);
  };
  const answerMutation = useMutation({
    mutationFn: () => api.answerInterview(id, current!.ordinal, answer.trim()),
    onSuccess: acceptResponse,
  });
  const skipMutation = useMutation({
    mutationFn: () => api.skipInterview(id, current!.ordinal),
    onSuccess: acceptResponse,
  });
  const retryPreparation = useMutation({
    mutationFn: () => api.retryInterviewPreparation(id),
    onSuccess: acceptResponse,
  });
  const retryDecision = useMutation({
    mutationFn: () => api.retryInterviewDecision(id),
    onSuccess: acceptResponse,
  });
  const fallbackDecision = useMutation({
    mutationFn: () => api.fallbackInterviewDecision(id),
    onSuccess: acceptResponse,
  });

  useEffect(() => {
    if (!stickToBottomRef.current) return;
    threadRef.current?.scrollTo({ top: threadRef.current.scrollHeight, behavior: "smooth" });
  }, [visibleTurns.length, current?.ordinal, session?.phase, answerMutation.isPending, skipMutation.isPending]);

  useEffect(() => {
    const node = composerRef.current;
    if (!node) return;
    node.style.height = "0px";
    node.style.height = `${Math.min(Math.max(node.scrollHeight, 44), 160)}px`;
  }, [answer, answering]);

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
          <p>{session.operation?.error_summary || "简历与历史记录没有受到影响，可以直接重试本次准备。"}</p>
          {retryPreparation.error ? <Alert title={normalizeError(retryPreparation.error).message} tone="danger" /> : null}
          <div className="button-row">
            <Button onClick={() => retryPreparation.mutate()} loading={retryPreparation.isPending}>重试本次准备</Button>
            <Link className="button button-secondary button-md" href="/interviews/new">重新配置</Link>
          </div>
        </Card>
      </div>
    );
  }
  if (session.status === "preparing" || session.phase === "preparing") {
    return (
      <div className="page narrow-page">
        <Card className="processing-state interview-preparing">
          <span className="processing-orb" />
          <h1>正在准备面试</h1>
          <p>正在结合简历、技术语言和目标公司规划能力覆盖，并生成第一个问题。</p>
          <div className="preparing-steps" aria-label="准备步骤">
            <span className="done">读取简历</span><span className="active">规划考察方向</span><span>进入面试</span>
          </div>
          <Link className="back-link" href="/interviews/records">稍后从面试记录返回</Link>
        </Card>
      </div>
    );
  }
  if (session.status === "completed") {
    return <div className="interview-loading"><Skeleton className="skeleton-card tall" /></div>;
  }

  const duration = session.question_duration_seconds || 180;
  const remaining = answering && now && enteredAt ? Math.max(0, duration - Math.floor((now - enteredAt) / 1000)) : duration;
  const overallElapsed = now ? elapsedSeconds(session.started_at ?? session.created_at, now) : 0;
  const mutationError = answerMutation.error || skipMutation.error || retryDecision.error || fallbackDecision.error;
  const busy = answerMutation.isPending || skipMutation.isPending;

  const submitAnswer = () => {
    if (!current || !answer.trim() || busy) return;
    answerMutation.mutate();
  };
  const onComposerKeyDown = (event: KeyboardEvent<HTMLTextAreaElement>) => {
    if (event.key !== "Enter" || event.shiftKey || event.nativeEvent.isComposing) return;
    event.preventDefault();
    submitAnswer();
  };

  return (
    <div className="interview-room interview-chat-room">
      <header className="interview-topbar">
        <Link href="/interviews/records"><ChevronLeft />退出</Link>
        <div className="timer-group">
          {answering ? <span className={remaining <= 30 ? "timer-warning" : ""}><Clock3 />本轮 {remaining ? formatTimer(remaining) : "已超时"}</span> : null}
          <span>整场 {formatTimer(overallElapsed)}</span>
        </div>
        <Badge tone={session.agent_mode === "ai" ? "success" : "warning"}>
          <Bot size={14} />{session.agent_mode === "ai" ? "实时 AI 面试官" : "规则降级模式"}
        </Badge>
      </header>

      <div
        className="interview-chat-stream"
        ref={threadRef}
        role="log"
        aria-live="polite"
        aria-relevant="additions"
        onScroll={() => {
          const node = threadRef.current;
          if (!node) return;
          stickToBottomRef.current = node.scrollHeight - node.scrollTop - node.clientHeight < 96;
        }}
      >
        <div className="interview-chat-thread">
          {visibleTurns.map((turn) => (
            <TurnThread
              key={turn.ordinal}
              turn={turn}
              isCurrent={turn.ordinal === current?.ordinal}
              pendingAnswer={turn.ordinal === current?.ordinal && answerMutation.isPending ? answer.trim() : undefined}
              pendingSkip={turn.ordinal === current?.ordinal && skipMutation.isPending}
            />
          ))}
          {deciding ? (
            <article className="chat-row interviewer">
              <span className="chat-avatar" aria-hidden="true"><Bot size={16} /></span>
              <div className="chat-bubble interviewer thinking">
                <span className="chat-meta">AI 面试官</span>
                <p className="chat-bubble-text"><span className="chat-typing" aria-hidden="true" /><span className="chat-typing" /><span className="chat-typing" />正在根据你的回答决定下一问</p>
              </div>
            </article>
          ) : null}
          {decisionFailed ? (
            <article className="chat-row interviewer">
              <span className="chat-avatar" aria-hidden="true"><Bot size={16} /></span>
              <div className="chat-bubble interviewer">
                <span className="chat-meta">系统</span>
                <p className="chat-bubble-text">{session.operation?.error_summary || "下一轮问题生成失败。你的回答已经保存，可以重试或使用系统兜底问题继续。"}</p>
              </div>
            </article>
          ) : null}
        </div>
      </div>

      <footer className="interview-composer">
        <div className="interview-composer-inner">
        {session.agent_mode !== "ai" ? <Alert title="当前不是 AI 实时提问" tone="warning">模型未启用，本场会明确使用规则与安全题材推进。</Alert> : null}
        {mutationError ? <Alert title={normalizeError(mutationError).message} tone="danger" /> : null}
        {decisionFailed ? (
          <div className="button-row composer-actions">
            <Button onClick={() => retryDecision.mutate()} loading={retryDecision.isPending}>重试下一轮决策</Button>
            <Button variant="secondary" onClick={() => fallbackDecision.mutate()} loading={fallbackDecision.isPending}>使用系统兜底问题继续</Button>
          </div>
        ) : (
          <>
            <label className="sr-only" htmlFor="answer">你的回答</label>
            <Textarea
              ref={composerRef}
              id="answer"
              value={busy ? "" : answer}
              onChange={(event) => setAnswer(event.target.value)}
              onKeyDown={onComposerKeyDown}
              placeholder={answering ? "输入回答，Enter 发送，Shift+Enter 换行" : deciding ? "面试官正在思考下一问，回答已保存" : "等待下一问…"}
              className="chat-composer-input"
              disabled={!answering || busy}
              rows={1}
            />
            <div className="composer-toolbar">
              <span className="draft-status">{answering ? (answer && !busy ? "草稿已保存在此设备" : busy ? "正在发送…" : "结合情境、行动和结果作答") : `已完成 ${visibleTurns.filter((turn) => Boolean(turn.answer) || turn.state === "skipped").length} 轮`}</span>
              <div className="button-row">
                <Button variant="secondary" onClick={() => skipMutation.mutate()} loading={skipMutation.isPending} disabled={!answering || answerMutation.isPending}>跳过</Button>
                <Button onClick={submitAnswer} loading={answerMutation.isPending} disabled={!answering || !answer.trim() || skipMutation.isPending}><Send size={16} />发送</Button>
              </div>
            </div>
          </>
        )}
        </div>
      </footer>
    </div>
  );
}

function TurnThread({
  turn,
  isCurrent,
  pendingAnswer,
  pendingSkip,
}: {
  turn: InterviewTurnResponse;
  isCurrent: boolean;
  pendingAnswer?: string;
  pendingSkip?: boolean;
}) {
  const followUp = isFollowUpTurn(turn);
  const skipped = turn.state === "skipped" || Boolean(turn.skipped_at) || Boolean(pendingSkip);
  const answered = Boolean(turn.answer) || Boolean(pendingAnswer);
  const candidateText = turn.answer || pendingAnswer || "";
  return (
    <div className="chat-turn">
      <article className="chat-row interviewer">
        <span className="chat-avatar" aria-hidden="true"><Bot size={16} /></span>
        <div className="chat-bubble interviewer">
          <span className="chat-meta">
            AI 面试官 · 第 {turn.ordinal} 轮
            {followUp ? <Badge tone="brand">追问</Badge> : <Badge tone="neutral">新问题</Badge>}
            {turn.difficulty ? <Badge tone="neutral">{turn.difficulty}</Badge> : null}
          </span>
          <ChatText text={turn.question} />
        </div>
      </article>
      {answered || skipped ? (
        <article className="chat-row candidate">
          <div className={`chat-bubble candidate${skipped && !answered ? " skipped" : ""}${pendingAnswer || pendingSkip ? " pending" : ""}`}>
            <span className="chat-meta">你{skipped && !answered ? " · 已跳过" : pendingAnswer ? " · 发送中" : ""}</span>
            {answered ? <ChatText text={candidateText} /> : <p className="chat-bubble-text muted-copy">本题已跳过，面试官将继续下一问。</p>}
          </div>
          <span className="chat-avatar candidate" aria-hidden="true"><UserRound size={16} /></span>
        </article>
      ) : isCurrent ? (
        <p className="chat-pending-hint">轮到你回答</p>
      ) : null}
    </div>
  );
}

function ChatText({ text }: { text: string }) {
  const [open, setOpen] = useState(false);
  const long = shouldCollapseChatText(text);
  const shown = !long || open ? text : `${text.slice(0, 360).trimEnd()}…`;
  return (
    <>
      <p className="chat-bubble-text">{shown}</p>
      {long ? (
        <button type="button" className="chat-expand" onClick={() => setOpen((value) => !value)}>
          {open ? "收起" : "展开全部"}
        </button>
      ) : null}
    </>
  );
}
