import Link from "next/link";
import type { Metadata } from "next";

export const metadata: Metadata = {
  title: "隐私说明",
};

export default function PrivacyPage() {
  return (
    <div className="marketing">
      <header className="marketing-nav">
        <Link className="product-brand" href="/"><span className="logo-mark">IM</span><span>InterviewMaster<small>AI 面试训练助手</small></span></Link>
        <div className="marketing-actions"><Link href="/login">登录</Link><Link className="button button-primary button-md" href="/register">免费开始</Link></div>
      </header>
      <main className="page narrow-page" style={{ padding: "48px 20px 80px" }}>
        <p className="eyebrow">法律与合规</p>
        <h1>隐私说明</h1>
        <p>InterviewMaster 用你的简历、职位描述和面试回答生成训练题目、追问和复盘报告。这些材料会发送给已配置的模型供应商处理，用于完成你发起的任务，而不是用于公开展示。</p>
        <h2>我们处理的数据</h2>
        <ul>
          <li>账户资料：邮箱、显示名称、密码哈希。</li>
          <li>训练材料：简历文件与抽取文本、面试提问、面试回答和评估记录。</li>
          <li>模型审计：调用时间、模型名、Prompt 版本、Token、费用估算和输入/输出哈希。默认不保存完整 Prompt 或回答正文。</li>
        </ul>
        <h2>第三方模型处理</h2>
        <p>启用 AI 后，简历、面试配置和回答会通过 OpenAI 兼容接口发送给供应商。生产环境应选择合同中明确不用于训练的账户或接口。评分为训练建议，不作为招聘结论。</p>
        <h2>你的控制权</h2>
        <ul>
          <li>在设置页导出账户数据。</li>
          <li>用当前密码删除账户；业务数据会级联删除，简历文件会尽量从对象存储移除。</li>
          <li>查看、修改或删除由报告生成的能力画像。</li>
        </ul>
        <h2>保留期限</h2>
        <p>模型调用审计记录默认保留 90 天。账户删除后不再用于新的模型调用。</p>
        <p><Link href="/">返回首页</Link></p>
      </main>
    </div>
  );
}
