// Run after a production build. API fixtures exercise Angular routes, forms,
// auth state and the M17 guard; real persistence/authorization is tested in Go.
const assert = require('node:assert/strict');
const http = require('node:http');
const fs = require('node:fs');
const path = require('node:path');
const { chromium } = require(process.env.PLAYWRIGHT_MODULE || 'playwright');

const root = path.resolve(__dirname, '../dist/frontend/browser');
const mime = { '.js': 'text/javascript', '.css': 'text/css', '.html': 'text/html', '.woff2': 'font/woff2', '.svg': 'image/svg+xml', '.png': 'image/png' };
const server = http.createServer((req, res) => {
  const pathname = new URL(req.url, 'http://localhost').pathname;
  const requested = path.resolve(root, '.' + pathname);
  const file = requested.startsWith(root + path.sep) && fs.existsSync(requested) && fs.statSync(requested).isFile()
    ? requested : path.join(root, 'index.html');
  res.setHeader('Content-Type', mime[path.extname(file)] || 'application/octet-stream');
  fs.createReadStream(file).pipe(res);
});

(async () => {
  await new Promise(resolve => server.listen(0, '127.0.0.1', resolve));
  const origin = `http://127.0.0.1:${server.address().port}`;
  const browser = await chromium.launch({ headless: true });
  let scenarios = 0;
  try {
    for (const role of ['STUDENT', 'COMPANY_SUPERVISOR', 'UNIVERSITY_SUPERVISOR', 'PROFESSOR', 'ADMIN']) {
      const statuses = role === 'COMPANY_SUPERVISOR' ? ['APPROVED', 'PENDING', 'REJECTED'] : [undefined];
      for (const status of statuses) {
        const context = await browser.newContext();
        await context.addInitScript(() => localStorage.setItem('internship_access_token', 'profile-smoke-token'));
        const page = await context.newPage();
        const errors = [];
        page.on('pageerror', error => errors.push(error.message));
        let user = { id: 21, role, fullName: 'کاربر آزمون', email: 'profile@example.test', phone: '09120000000',
          ...(role === 'STUDENT' ? { studentNumber: '40123456', major: 'مهندسی کامپیوتر' } : {}),
          ...(status ? { jobTitle: 'سرپرست', companyId: 17, companyName: 'شرکت آزمون', companyRegistrationStatus: status } : {}) };
        let updates = 0;
        let passwordChanges = 0;
        const fulfill = (route, body, code = 200) => route.fulfill({ status: code, contentType: 'application/json', body: JSON.stringify(body) });
        await page.route('**/api/**', async route => {
          const request = route.request();
          const endpoint = new URL(request.url()).pathname;
          if (endpoint === '/api/auth/me') return fulfill(route, user);
          if (endpoint === '/api/profile' && request.method() === 'PUT') {
            const input = request.postDataJSON();
            assert.deepEqual(Object.keys(input).sort(), (status ? ['fullName', 'phone', 'jobTitle'] : ['fullName', 'phone']).sort());
            updates++;
            user = { ...user, ...input };
            return fulfill(route, user);
          }
          if (endpoint === '/api/profile/change-password') {
            const input = request.postDataJSON();
            assert.deepEqual(Object.keys(input).sort(), ['currentPassword', 'newPassword']);
            passwordChanges++;
            if (input.currentPassword !== 'Current123!') return fulfill(route, { error: 'رمز عبور فعلی نادرست است.' }, 400);
            return fulfill(route, { message: 'رمز عبور با موفقیت تغییر کرد.' });
          }
          if (endpoint === '/api/company/profile') return fulfill(route, {
            company: { id: 17, name: 'شرکت آزمون', nationalId: 'legal-17', economicCode: 'economic-17', registrationStatus: status, isApproved: false }, supervisor: user
          });
          return fulfill(route, []);
        });
        await page.goto(origin + '/profile');
        await page.getByRole('heading', { name: 'پروفایل من', exact: true }).waitFor();
        await page.locator('#profile-name').waitFor();
        assert.equal(await page.locator('.detail').filter({ hasText: 'ایمیل (فقط خواندنی)' }).locator('input').count(), 0);
        assert.equal(await page.locator('#profile-title').count(), status ? 1 : 0);
        if (role === 'STUDENT') {
          assert.match(await page.locator('.detail-grid').innerText(), /40123456/);
          assert.match(await page.locator('.detail-grid').innerText(), /مهندسی کامپیوتر/);
          assert.match(await page.locator('.hint').innerText(), /پرونده‌های کارآموزی/);
        }
        if (status) assert.match(await page.locator('.detail-grid').innerText(), /شرکت آزمون/);
        await page.locator('#profile-name').fill('   ');
        assert.equal(await page.getByRole('button', { name: 'ذخیره اطلاعات', exact: true }).isDisabled(), true);
        await page.locator('#profile-name').fill('نام به‌روز');
        await page.locator('#profile-phone').fill('تماس جدید');
        if (status) await page.locator('#profile-title').fill('عنوان جدید');
        await page.getByRole('button', { name: 'ذخیره اطلاعات', exact: true }).click();
        await page.waitForFunction(() => document.querySelector('.identity strong')?.textContent === 'نام به‌روز');
        assert.equal(updates, 1);
        await page.locator('#current-password').fill('Wrong123!');
        await page.locator('#new-password').fill('NewPassword123!');
        await page.locator('#confirm-password').fill('Mismatch123!');
        const passwordButton = page.getByRole('button', { name: 'تغییر رمز عبور', exact: true });
        assert.equal(await passwordButton.isDisabled(), true);
        assert.equal(passwordChanges, 0);
        await page.locator('#confirm-password').fill('NewPassword123!');
        await passwordButton.click();
        await page.getByText('رمز عبور فعلی نادرست است.', { exact: true }).waitFor();
        assert.equal(new URL(page.url()).pathname, '/profile');
        assert.equal(await page.evaluate(() => localStorage.getItem('internship_access_token')), 'profile-smoke-token');
        await page.locator('#current-password').fill('Current123!');
        await passwordButton.click();
        await page.waitForFunction(() => document.querySelector('#current-password')?.value === '');
        assert.equal(await page.locator('#new-password').inputValue(), '');
        assert.equal(await page.locator('#confirm-password').inputValue(), '');
        assert.equal(passwordChanges, 2);
        if (status && status !== 'APPROVED') {
          for (const operational of ['/company/opportunities', '/company/internships', '/company/opportunities/1/applications']) {
            await page.goto(origin + operational);
            await page.waitForURL(origin + '/company/profile');
          }
          await page.getByRole('link', { name: 'پروفایل من', exact: true }).click();
          await page.waitForURL(origin + '/profile');
          await page.locator('#profile-name').waitFor();
          await page.getByRole('link', { name: 'ثبت / پروفایل شرکت', exact: true }).click();
          await page.waitForURL(origin + '/company/profile');
        }
        assert.deepEqual(errors, []);
        console.log(`PASS ${role}${status ? ' / ' + status : ''}: profile, edit, header, password${status && status !== 'APPROVED' ? ', restricted navigation' : ''}`);
        scenarios++;
        await context.close();
      }
    }
    const guest = await browser.newPage();
    await guest.goto(origin + '/profile');
    await guest.waitForURL(origin + '/login');
    console.log(`PASS unauthenticated redirect; ${scenarios} authenticated role/status scenarios`);
  } finally {
    await browser.close();
    await new Promise(resolve => server.close(resolve));
  }
})().catch(error => { console.error(error); server.close(); process.exitCode = 1; });
