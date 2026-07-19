"use client";

import { FormEvent, useState } from "react";

type Json = Record<string, unknown>;
const api = async (path: string, init: RequestInit = {}) => {
  const token = typeof window === "undefined" ? "" : localStorage.getItem("im_token") ?? "";
  const response = await fetch(path, { ...init, headers: { "Content-Type": "application/json", ...(token ? { Authorization: `Bearer ${token}` } : {}), ...init.headers } });
  const body = await response.json(); if (!response.ok) throw new Error(body.message ?? `HTTP ${response.status}`); return body;
};

export default function Home() {
  const [token,setToken]=useState(""); const [result,setResult]=useState<Json|Json[]|null>(null); const [error,setError]=useState("");
  const [resumeId,setResumeId]=useState(""); const [questionSetId,setQuestionSetId]=useState(""); const [sessionId,setSessionId]=useState("");
  const [answer,setAnswer]=useState("");
  const run=async(fn:()=>Promise<Json|Json[]>)=>{setError("");try{setResult(await fn())}catch(e){setError(e instanceof Error?e.message:String(e))}};
  const auth=async(e:FormEvent<HTMLFormElement>)=>{e.preventDefault();const f=new FormData(e.currentTarget);const body=await api("/api/v1/auth/login",{method:"POST",body:JSON.stringify({email:f.get("email"),password:f.get("password")})});const value=String(body.access_token);localStorage.setItem("im_token",value);setToken(value);setResult(body)};
  const createJD=(e:FormEvent<HTMLFormElement>)=>{e.preventDefault();const f=new FormData(e.currentTarget);void run(()=>api("/api/v1/job-descriptions",{method:"POST",body:JSON.stringify({company:f.get("company"),title:f.get("title"),content:f.get("content")})}))};
  const uploadResume=async(e:FormEvent<HTMLFormElement>)=>{e.preventDefault();const f=new FormData(e.currentTarget);const file=f.get("resume") as File;if(!file)return;setError("");try{const ticket=await api("/api/v1/resumes/uploads",{method:"POST",body:JSON.stringify({title:file.name,file_name:file.name,content_type:file.type||"application/octet-stream",size_bytes:file.size})});const put=await fetch(String(ticket.upload_url),{method:"PUT",headers:{"Content-Type":file.type||"application/octet-stream"},body:file});if(!put.ok)throw new Error("对象存储上传失败");const id=String(ticket.resume_id),version=String(ticket.version_id);setResumeId(id);setResult(await api(`/api/v1/resumes/${id}/versions/${version}/complete`,{method:"POST"}))}catch(x){setError(x instanceof Error?x.message:String(x))}};
  const transcribe=async(e:FormEvent<HTMLFormElement>)=>{e.preventDefault();const f=new FormData(e.currentTarget);const file=f.get("audio") as File;if(!file)return;setError("");try{const ticket=await api("/api/v1/beta/asr/uploads",{method:"POST",body:JSON.stringify({file_name:file.name,content_type:file.type||"audio/wav",size_bytes:file.size})});const put=await fetch(String(ticket.upload_url),{method:"PUT",headers:{"Content-Type":file.type||"audio/wav"},body:file});if(!put.ok)throw new Error("音频上传失败");const task=await api("/api/v1/beta/asr/tasks",{method:"POST",body:JSON.stringify({object_key:ticket.object_key,language:"zh"})});let state:Json=task;for(let i=0;i<180;i++){state=await api(`/api/v1/tasks/${String(task.task_id)}`);if(state.status==="succeeded"||state.status==="failed")break;await new Promise(resolve=>setTimeout(resolve,1000))}setResult(state)}catch(x){setError(x instanceof Error?x.message:String(x))}};
  const questions=()=>run(async()=>{const x=await api("/api/v1/question-sets",{method:"POST",body:JSON.stringify({resume_id:resumeId,target_role:"目标岗位"})});setQuestionSetId(String(x.id));return x});
  const interview=()=>run(async()=>{const x=await api("/api/v1/interviews",{method:"POST",body:JSON.stringify({resume_id:resumeId,question_set_id:questionSetId,title:"文字模拟面试"})});setSessionId(String(x.id));return x});
  return <main className="workspace"><header className="topbar"><span className="brand"><b className="brandMark">IM</b>InterviewMaster</span><span>MVP + BETA WORKSPACE</span></header>
    <section className="hero compactHero"><div><p className="eyebrow">AI INTERVIEW PRACTICE</p><h1>从简历事实到面试复盘，<span>完成一轮可追溯训练。</span></h1></div></section>
    <section className="workGrid">
      <article className="workCard"><h2>1. 登录</h2><form onSubmit={auth}><input name="email" type="email" placeholder="邮箱" required/><input name="password" type="password" placeholder="密码" required/><button>登录</button></form><small>{token?"已登录":"也可调用 /auth/register 注册"}</small></article>
      <article className="workCard"><h2>2. 岗位 JD</h2><form onSubmit={createJD}><input name="company" placeholder="公司"/><input name="title" placeholder="岗位" required/><textarea name="content" placeholder="粘贴至少 20 字 JD" required/><button>保存并提取能力</button></form></article>
      <article className="workCard"><h2>3. 简历与题集</h2><form onSubmit={uploadResume}><input name="resume" type="file" accept=".pdf,.docx,.txt" required/><button>上传并解析</button></form><input value={resumeId} onChange={e=>setResumeId(e.target.value)} placeholder="resume_id"/><button onClick={questions}>生成题集</button><input value={questionSetId} onChange={e=>setQuestionSetId(e.target.value)} placeholder="question_set_id"/></article>
      <article className="workCard"><h2>4. 文字模拟与报告</h2><button onClick={interview}>创建面试</button><input value={sessionId} onChange={e=>setSessionId(e.target.value)} placeholder="session_id"/><textarea value={answer} onChange={e=>setAnswer(e.target.value)} placeholder="输入当前问题的回答"/><button onClick={()=>run(()=>api(`/api/v1/interviews/${sessionId}/answer`,{method:"POST",body:JSON.stringify({answer})}))}>提交当前回答</button><button onClick={()=>run(()=>api(`/api/v1/interviews/${sessionId}`))}>刷新会话</button><button onClick={()=>run(()=>api(`/api/v1/interviews/${sessionId}/report`))}>生成报告</button></article>
      <article className="workCard"><h2>5. Beta 面经与语音</h2><button onClick={()=>run(()=>api("/api/v1/beta/company-intel/search",{method:"POST",body:JSON.stringify({company:"目标公司",role:"后端工程师"})}))}>查询面经画像</button><form onSubmit={transcribe}><input name="audio" type="file" accept="audio/*,.wav,.mp3,.m4a,.webm,.ogg" required/><button>上传并转写语音</button></form></article>
      <article className="workCard output"><h2>运行结果</h2>{error&&<p className="errorText">{error}</p>}<pre>{result?JSON.stringify(result,null,2):"等待操作"}</pre></article>
    </section></main>;
}
