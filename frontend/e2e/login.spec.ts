import { test, expect } from '@playwright/test';

test.describe('Public pages', () => {
  test('login page loads and shows login form', async ({ page }) => {
    await page.goto('/login');

    // Verify login page elements
    await expect(page.locator('h1, h2, h3').first()).toBeVisible();
    await expect(page.getByPlaceholder(/用户名|账号|username/i).first()).toBeVisible({ timeout: 10000 });

    // Verify form fields exist
    const usernameInput = page.getByPlaceholder(/用户名|账号|username/i).first();
    const passwordInput = page.getByPlaceholder(/密码|password/i).first();
    const loginButton = page.getByRole('button', { name: /登录|login|提交/i }).first();

    await expect(usernameInput).toBeVisible();
    await expect(passwordInput).toBeVisible();
    await expect(loginButton).toBeVisible();
  });

  test('login with invalid credentials shows error', async ({ page }) => {
    await page.goto('/login');

    const usernameInput = page.getByPlaceholder(/用户名|账号|username/i).first();
    const passwordInput = page.getByPlaceholder(/密码|password/i).first();
    const loginButton = page.getByRole('button', { name: /登录|login|提交/i }).first();

    await usernameInput.fill('wrong_user');
    await passwordInput.fill('wrong_pass');
    await loginButton.click();

    // Should show an error message (either inline or toast)
    await expect(page.getByText(/错误|失败|无效|invalid|error/i).first()).toBeVisible({ timeout: 10000 });
  });
});

test.describe('Dashboard', () => {
  test('redirects to login when not authenticated', async ({ page }) => {
    await page.goto('/dashboard');
    // Should redirect to login
    await expect(page).toHaveURL(/login/);
  });
});

test.describe('Student flow', () => {
  test('student can see tasks page after login', async ({ page }) => {
    await page.goto('/login');

    // Login as student — the backend uses seeded JWT tokens
    // Since the frontend calls the real API, we need a running backend.
    // This test checks the login flow reaches the API.
    const usernameInput = page.getByPlaceholder(/用户名|账号|username/i).first();
    const passwordInput = page.getByPlaceholder(/密码|password/i).first();
    const loginButton = page.getByRole('button', { name: /登录|login|提交/i }).first();

    await usernameInput.fill('student_a');
    await passwordInput.fill('password123');
    await loginButton.click();

    // Wait for navigation after login
    await page.waitForURL(/\/(dashboard|student)/, { timeout: 15000 });
  });
});