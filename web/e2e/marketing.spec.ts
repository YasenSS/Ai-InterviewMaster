import { expect, test } from "@playwright/test";

test("官网展示核心价值与主操作", async ({ page }) => {
  await page.goto("/");
  await expect(page.getByRole("heading", { level: 1 })).toContainText("AI 模拟面试");
  await expect(page.getByRole("link", { name: /免费开始/ }).first()).toBeVisible();
  const overflow = await page.evaluate(() => ({
    clientWidth: document.documentElement.clientWidth,
    scrollWidth: document.documentElement.scrollWidth,
  }));
  expect(overflow.scrollWidth).toBe(overflow.clientWidth);
});

test("受保护路由安全返回登录页", async ({ page }) => {
  await page.goto("/resumes");
  await expect(page).toHaveURL(/\/login\?return_to=/);
});
