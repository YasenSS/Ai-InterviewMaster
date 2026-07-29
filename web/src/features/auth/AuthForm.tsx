"use client";

import { zodResolver } from "@hookform/resolvers/zod";
import { useMutation, useQueryClient } from "@tanstack/react-query";
import { ArrowLeft } from "lucide-react";
import Link from "next/link";
import { useRouter, useSearchParams } from "next/navigation";
import { useForm } from "react-hook-form";
import { z } from "zod";

import { Alert } from "@/components/feedback/States";
import { Button } from "@/components/ui/Button";
import { FormField, Input, PasswordInput } from "@/components/ui/Form";
import { api } from "@/shared/api/services";
import { authStorage, normalizeError } from "@/shared/api/client";
import { queryKeys } from "@/shared/api/query";
import { safeReturnTo } from "@/shared/lib/utils";

const loginSchema = z.object({
  email: z.email("请输入有效的邮箱地址"),
  password: z.string().min(1, "请输入密码"),
});

const registerSchema = z
  .object({
    displayName: z.string().trim().min(2, "显示名称至少 2 个字符").max(50, "显示名称不能超过 50 个字符"),
    email: z.email("请输入有效的邮箱地址"),
    password: z.string().min(8, "密码至少 8 个字符").max(72, "密码不能超过 72 个字符"),
    confirmPassword: z.string(),
  })
  .refine((value) => value.password === value.confirmPassword, {
    message: "两次输入的密码不一致",
    path: ["confirmPassword"],
  });

type LoginValues = z.infer<typeof loginSchema>;
type RegisterValues = z.infer<typeof registerSchema>;

export function AuthForm({ mode }: { mode: "login" | "register" }) {
  const router = useRouter();
  const search = useSearchParams();
  const queryClient = useQueryClient();
  const isLogin = mode === "login";
  const loginForm = useForm<LoginValues>({ resolver: zodResolver(loginSchema), mode: "onBlur" });
  const registerForm = useForm<RegisterValues>({ resolver: zodResolver(registerSchema), mode: "onBlur" });
  const mutation = useMutation({
    mutationFn: async (values: LoginValues | RegisterValues) => {
      if ("displayName" in values) {
        return api.register({
          display_name: values.displayName,
          email: values.email,
          password: values.password,
        });
      }
      return api.login(values);
    },
    onSuccess: (response) => {
      authStorage.set(response.access_token);
      queryClient.setQueryData(queryKeys.me, response.user);
      router.replace(safeReturnTo(search.get("return_to")));
    },
    onError: (error) => {
      const appError = normalizeError(error);
      const fields = appError.fieldErrors;
      if (!isLogin && (appError.code.includes("EMAIL") || fields?.email)) {
        registerForm.setError("email", { message: "该邮箱已被使用，请直接登录或更换邮箱。" }, { shouldFocus: true });
      }
    },
  });

  const appError = mutation.error ? normalizeError(mutation.error) : null;
  return (
    <div className="auth-page">
      <div className="auth-aside">
        <Link className="product-brand light" href="/"><span className="logo-mark">IM</span>InterviewMaster</Link>
        <blockquote>“真正有效的练习，不是重复回答，而是知道下一次应该改什么。”</blockquote>
        <p>基于真实简历和目标岗位，让每轮面试都有依据、节奏与复盘。</p>
      </div>
      <main className="auth-main">
        <div className="auth-card">
          <Link className="back-link" href="/"><ArrowLeft size={17} /> 返回官网</Link>
          <div className="auth-heading"><p className="eyebrow">{isLogin ? "欢迎回来" : "开始第一轮训练"}</p><h1>{isLogin ? "登录 InterviewMaster" : "创建你的账户"}</h1><p>{isLogin ? "继续未完成的训练，或开始新一轮模拟面试。" : "注册后从上传简历开始，逐步完成一次可复盘的模拟面试。"}</p></div>
          {appError && !(!isLogin && appError.code.includes("EMAIL")) ? <Alert title={isLogin && appError.status === 401 ? "邮箱或密码不正确" : appError.message} tone="danger">{appError.requestId ? `请求 ID：${appError.requestId}` : null}</Alert> : null}
          {isLogin ? (
            <form className="form-stack" onSubmit={loginForm.handleSubmit((values) => mutation.mutate(values))} noValidate>
              <FormField label="邮箱" htmlFor="email" error={loginForm.formState.errors.email?.message}><Input id="email" type="email" autoComplete="email" aria-invalid={Boolean(loginForm.formState.errors.email)} {...loginForm.register("email")} /></FormField>
              <FormField label="密码" htmlFor="password" error={loginForm.formState.errors.password?.message}><PasswordInput id="password" autoComplete="current-password" aria-invalid={Boolean(loginForm.formState.errors.password)} {...loginForm.register("password")} /></FormField>
              <Button type="submit" loading={mutation.isPending}>登录</Button>
            </form>
          ) : (
            <form className="form-stack" onSubmit={registerForm.handleSubmit((values) => mutation.mutate(values))} noValidate>
              <FormField label="显示名称" htmlFor="displayName" error={registerForm.formState.errors.displayName?.message}><Input id="displayName" autoComplete="name" {...registerForm.register("displayName")} /></FormField>
              <FormField label="邮箱" htmlFor="email" error={registerForm.formState.errors.email?.message}><Input id="email" type="email" autoComplete="email" {...registerForm.register("email")} /></FormField>
              <FormField label="密码" hint="至少 8 个字符" htmlFor="password" error={registerForm.formState.errors.password?.message}><PasswordInput id="password" autoComplete="new-password" {...registerForm.register("password")} /></FormField>
              <FormField label="确认密码" htmlFor="confirmPassword" error={registerForm.formState.errors.confirmPassword?.message}><PasswordInput id="confirmPassword" autoComplete="new-password" {...registerForm.register("confirmPassword")} /></FormField>
              <Button type="submit" loading={mutation.isPending}>免费注册</Button>
            </form>
          )}
          <p className="auth-switch">{isLogin ? "还没有账户？" : "已经有账户？"} <Link href={isLogin ? "/register" : "/login"}>{isLogin ? "免费注册" : "直接登录"}</Link></p>
        </div>
      </main>
    </div>
  );
}
