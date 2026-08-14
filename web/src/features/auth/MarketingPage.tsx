import {
  ArrowRight,
  BarChart3,
  Building2,
  FileSearch,
  MessagesSquare,
  Sparkles,
} from "lucide-react";
import Link from "next/link";

const capabilities = [
  { icon: FileSearch, title: "理解真实经历", text: "从简历中提取项目、职责和技能，让每一次提问都有你的经历作为依据。" },
  { icon: Building2, title: "匹配目标方向", text: "结合主技术语言、后端开发岗位和目标公司，调整本场面试的考察重点。" },
  { icon: MessagesSquare, title: "动态追问", text: "模型根据你的回答选择继续深挖、切换考点或结束，让对话更接近真实面试。" },
  { icon: BarChart3, title: "完整复盘", text: "保留每一次提问和回答，并给出逐题评分、点评与可借鉴的改进答案。" },
];

export function MarketingPage() {
  return (
    <div className="marketing">
      <header className="marketing-nav">
        <Link className="product-brand" href="/"><span className="logo-mark">IM</span><span>InterviewMaster<small>AI 面试训练助手</small></span></Link>
        <nav aria-label="官网导航"><a href="#features">核心能力</a><a href="#process">使用流程</a></nav>
        <div className="marketing-actions"><Link href="/login">登录</Link><Link className="button button-primary button-md" href="/register">免费开始</Link></div>
      </header>
      <main>
        <section className="marketing-hero">
          <div className="hero-copy">
            <p className="eyebrow"><Sparkles size={15} /> 基于真实经历的动态面试</p>
            <h1>从你的简历出发，完成一场会追问、能复盘的<span>AI 模拟面试。</span></h1>
            <p className="hero-lede">选择主技术语言和目标公司，立即进入后端开发面试。面试官会沿着你的回答持续深挖，而不是机械地念完固定问题。</p>
            <div className="hero-actions"><Link className="button button-primary button-lg" href="/register">免费开始训练<ArrowRight size={18} /></Link><a className="button button-secondary button-lg" href="#process">了解如何使用</a></div>
            <p className="hero-note">无需信用卡 · 支持 PDF、DOCX 和 TXT 简历</p>
          </div>
          <div className="hero-demo" aria-label="产品界面示意">
            <div className="demo-top"><span /><span /><span /><small>模拟面试 · 第 3 轮</small></div>
            <div className="demo-body">
              <span className="badge badge-brand">项目经历 · 追问</span>
              <h2>你刚才提到把接口延迟降低了 40%，具体定位到了哪个瓶颈？</h2>
              <p>请说明判断依据、验证过程，以及最终采用这个方案的原因。</p>
              <div className="demo-input">我先通过链路追踪确认了……<span>02:14</span></div>
            </div>
          </div>
        </section>
        <section className="process-section" id="process">
          <div className="section-intro"><p className="eyebrow">三步完成一次训练</p><h2>准备更少，练得更深</h2></div>
          <ol className="steps">
            <li><span>01</span><h3>上传简历</h3><p>解析你的项目经历、技术能力和可继续深挖的事实。</p></li>
            <li><span>02</span><h3>选择方向</h3><p>选择 Java、Go 或 C++ 等主语言，以及本次求职的目标公司。</p></li>
            <li><span>03</span><h3>直接开场</h3><p>进入动态面试，结束后回看完整问答、评分与改进答案。</p></li>
          </ol>
        </section>
        <section className="capabilities-section" id="features">
          <div className="section-intro"><p className="eyebrow">完整训练闭环</p><h2>不只问问题，更帮助你理解怎样答得更好</h2></div>
          <div className="capability-grid">{capabilities.map(({ icon: Icon, title, text }) => <article key={title}><span><Icon /></span><h3>{title}</h3><p>{text}</p></article>)}</div>
        </section>
        <section className="scenario-section">
          <article><p className="eyebrow">应届求职</p><h2>第一次面对正式面试，也能提前建立清晰的表达框架。</h2></article>
          <article><p className="eyebrow">社招进阶</p><h2>让模型沿着项目细节持续追问，发现经历中还没有讲透的部分。</h2></article>
        </section>
        <section className="final-cta"><h2>让下一次面试，从一次真正有来有回的练习开始。</h2><p>上传简历，选择技术语言和目标公司，开始第一场 AI 模拟面试。</p><Link className="button button-primary button-lg" href="/register">免费开始<ArrowRight size={18} /></Link></section>
      </main>
      <footer className="marketing-footer"><Link className="product-brand" href="/"><span className="logo-mark">IM</span>InterviewMaster</Link><p>© 2026 InterviewMaster · AI 面试训练助手</p><div><Link href="/privacy">隐私说明</Link><Link href="/login">登录</Link><Link href="/register">注册</Link></div></footer>
    </div>
  );
}
