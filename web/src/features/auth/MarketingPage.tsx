import {
  ArrowRight,
  BarChart3,
  ClipboardCheck,
  FileSearch,
  MessagesSquare,
  Sparkles,
} from "lucide-react";
import Link from "next/link";

const capabilities = [
  { icon: FileSearch, title: "简历理解", text: "从真实经历中提取结构化事实，让问题有材料依据。" },
  { icon: ClipboardCheck, title: "定制题集", text: "结合可选 JD 与目标岗位，生成更贴近方向的问题。" },
  { icon: MessagesSquare, title: "逐题面试", text: "一次专注一道题，在真实节奏中组织和打磨表达。" },
  { icon: BarChart3, title: "复盘报告", text: "从评分、点评和证据中找到下一轮训练的优先级。" },
];

export function MarketingPage() {
  return (
    <div className="marketing">
      <header className="marketing-nav">
        <Link className="product-brand" href="/"><span className="logo-mark">IM</span><span>InterviewMaster<small>AI 面试训练助手</small></span></Link>
        <nav aria-label="官网导航">
          <a href="#features">核心能力</a><a href="#process">使用流程</a>
        </nav>
        <div className="marketing-actions"><Link href="/login">登录</Link><Link className="button button-primary button-md" href="/register">免费开始</Link></div>
      </header>
      <main>
        <section className="marketing-hero">
          <div className="hero-copy">
            <p className="eyebrow"><Sparkles size={15} /> 基于真实材料的刻意练习</p>
            <h1>基于你的简历和目标岗位，完成可复盘的 <span>AI 模拟面试。</span></h1>
            <p className="hero-lede">从准备材料、逐题作答到结构化复盘，把每一次练习都变成看得见的进步。</p>
            <div className="hero-actions"><Link className="button button-primary button-lg" href="/register">免费开始训练 <ArrowRight size={18} /></Link><a className="button button-secondary button-lg" href="#process">了解如何使用</a></div>
            <p className="hero-note">无需信用卡 · 支持 PDF、DOCX 和 TXT 简历</p>
          </div>
          <div className="hero-demo" aria-label="产品界面示意">
            <div className="demo-top"><span /><span /><span /><small>模拟面试 · 第 3 / 8 题</small></div>
            <div className="demo-body">
              <span className="badge badge-brand">项目经历</span>
              <h2>请讲述一个你主动推动复杂项目落地的经历。</h2>
              <p>可以从目标、协作难点、你的具体行动和最终结果展开。</p>
              <div className="demo-input">在这个项目中，我首先……<span>02:14</span></div>
            </div>
          </div>
        </section>
        <section className="process-section" id="process">
          <div className="section-intro"><p className="eyebrow">三步完成一次训练</p><h2>准备更少，练得更深</h2></div>
          <ol className="steps">
            <li><span>01</span><h3>准备材料</h3><p>上传简历，并按需添加目标岗位的 JD。</p></li>
            <li><span>02</span><h3>开始面试</h3><p>生成定制题集，在计时环境中逐题作答。</p></li>
            <li><span>03</span><h3>获得报告</h3><p>查看逐题评分、证据与下一步训练建议。</p></li>
          </ol>
        </section>
        <section className="capabilities-section" id="features">
          <div className="section-intro"><p className="eyebrow">完整训练闭环</p><h2>不只生成问题，更帮助你复盘</h2></div>
          <div className="capability-grid">{capabilities.map(({ icon: Icon, title, text }) => <article key={title}><span><Icon /></span><h3>{title}</h3><p>{text}</p></article>)}</div>
        </section>
        <section className="scenario-section">
          <article><p className="eyebrow">应届求职</p><h2>第一次面对正式面试，也能提前建立表达框架。</h2></article>
          <article><p className="eyebrow">社招进阶</p><h2>把复杂项目经验，转化为更清晰、有证据的回答。</h2></article>
        </section>
        <section className="final-cta"><h2>让下一次面试，从一次认真练习开始。</h2><p>上传你的简历，开始第一轮 AI 模拟面试。</p><Link className="button button-primary button-lg" href="/register">免费开始 <ArrowRight size={18} /></Link></section>
      </main>
      <footer className="marketing-footer"><Link className="product-brand" href="/"><span className="logo-mark">IM</span>InterviewMaster</Link><p>© 2026 InterviewMaster · AI 面试训练助手</p><div><Link href="/login">登录</Link><Link href="/register">注册</Link></div></footer>
    </div>
  );
}
