const assert = require('node:assert/strict');
const http = require('node:http');
const fs = require('node:fs');
const path = require('node:path');
const { chromium } = require(process.env.PLAYWRIGHT_MODULE || 'playwright');

const root = path.resolve(__dirname, '../dist/frontend/browser');
const mime = { '.js': 'text/javascript', '.css': 'text/css', '.woff2': 'font/woff2', '.svg': 'image/svg+xml' };
const server = http.createServer((req, res) => {
  const requested = path.resolve(root, '.' + new URL(req.url, 'http://localhost').pathname);
  const file = requested.startsWith(root + path.sep) && fs.existsSync(requested) && fs.statSync(requested).isFile()
    ? requested : path.join(root, 'index.html');
  res.setHeader('Content-Type', mime[path.extname(file)] || 'text/html');
  fs.createReadStream(file).pipe(res);
});

const date = '2026-10-01T10:00:00Z';
const applicationLabels = { PENDING: 'در انتظار بررسی', ACCEPTED: 'پذیرفته شده', REJECTED: 'رد شده' };
const caseLabels = { ACTIVE: 'در حال انجام کارآموزی', PASSED: 'قبول شده', FAILED: 'مردود', CANCELLED: 'لغو شده' };
const companies = [
  { id: 1, name: 'شرکت جدید', nationalId: '101', economicCode: '201', isApproved: false, passedInternshipCount: 1 },
  { id: 2, name: 'شرکت تأییدشده', nationalId: '102', economicCode: '202', isApproved: true }
];
const applications = Object.keys(applicationLabels).map((status, index) => ({
  id: index + 1, status, appliedAt: date, reviewedAt: date,
  opportunity: { id: 1, title: `فرصت ${status}`, company: companies[0] },
  student: { id: index + 1, fullName: `متقاضی ${status}`, email: `${index}@example.test` }
}));
const opportunities = ['OPEN', 'CLOSED'].map((status, index) => ({
  id: index + 1, status, title: `فرصت ${status}`, workField: 'نرم‌افزار', location: 'تهران', createdAt: date
}));
const terms = ['OPEN', 'CLOSED'].map((status, index) => ({
  id: index + 1, status, academicYear: 1405 - index, termType: 'FIRST', openedAt: date, closedAt: null
}));
function internshipCase(status, index) {
  const preference = { application: { opportunity: { company: companies[0], title: `اولویت ${status}` } } };
  return {
    id: index + 1, caseId: index + 1, status, term: terms[0],
    student: { fullName: `دانشجو ${status} ${index}`, studentNumber: String(1000 + index) },
    professor: { fullName: 'استاد آزمون' }, preferences: [preference],
    selectedPreference: status === 'PENDING_UNIVERSITY_REVIEW' ? null : preference,
    createdAt: date, submittedAt: date, canProfessorComplete: false
  };
}
const cases = ['PENDING_UNIVERSITY_REVIEW', 'PENDING_COMPANY_DETAILS', 'PENDING_FINAL_APPROVAL', ...Object.keys(caseLabels)].map(internshipCase);
const termCases = [...Array.from({ length: 12 }, (_, i) => internshipCase('ACTIVE', i)), internshipCase('CANCELLED', 12), internshipCase('DRAFT', 13)];
const users = [
  { id: 31, role: 'STUDENT', fullName: 'دانشجوی فعال', email: 'active@example.test', isActive: true, verificationStatus: 'PENDING' },
  { id: 32, role: 'PROFESSOR', fullName: 'استاد غیرفعال', email: 'inactive@example.test', isActive: false, verificationStatus: 'APPROVED' }
];

(async () => {
  await new Promise(resolve => server.listen(0, '127.0.0.1', resolve));
  const origin = `http://127.0.0.1:${server.address().port}`;
  let browser;
  let checks = 0;
  const errors = [];
  try {
    browser = await chromium.launch({ headless: true, ...(process.env.CHROME_PATH ? { executablePath: process.env.CHROME_PATH } : {}) });
    async function scenario(role, url, test) {
      const context = await browser.newContext();
      try {
        await context.addInitScript(() => localStorage.setItem('internship_access_token', 'status-filter-smoke'));
        const page = await context.newPage();
        page.on('pageerror', error => errors.push(error.message));
        const requests = [];
        await page.route('**/api/**', route => {
          const url = new URL(route.request().url());
          const endpoint = url.pathname;
          requests.push(url);
          let body;
          if (endpoint === '/api/auth/me') body = { id: 20, role, fullName: 'کاربر آزمون', email: 'test@example.test', isActive: true, verificationStatus: 'APPROVED', companyRegistrationStatus: 'APPROVED' };
          else if (endpoint === '/api/notifications/unread-count') body = { count: 0 };
          else if (endpoint === '/api/student/opportunity-applications' || endpoint === '/api/company/opportunities/1/applications') body = applications;
          else if (endpoint === '/api/company/opportunities') body = opportunities;
          else if (endpoint === '/api/company/opportunities/1') body = opportunities[0];
          else if (endpoint === '/api/university/internship-cases') body = cases.filter(item => !url.searchParams.get('status') || item.status === url.searchParams.get('status'));
          else if (endpoint === '/api/company/internship-cases' || endpoint === '/api/professor/internship-cases') body = cases.filter(item => endpoint.includes('/company/') ? item.status !== 'PENDING_UNIVERSITY_REVIEW' : item.status in caseLabels);
          else if (endpoint === '/api/university/internship-terms') body = terms;
          else if (/^\/api\/university\/internship-terms\/\d+$/.test(endpoint)) body = { term: terms[Number(endpoint.split('/').pop()) - 1], cases: termCases };
          else if (endpoint === '/api/university/companies') body = companies;
          else if (endpoint === '/api/university/companies/eligible-for-approval') body = [companies[0]];
          else if (endpoint === '/api/university/companies/approved') body = [companies[1]];
          else if (endpoint === '/api/admin/users') body = users.filter(item => (!url.searchParams.get('role') || item.role === url.searchParams.get('role')) && (!url.searchParams.get('isActive') || String(item.isActive) === url.searchParams.get('isActive')));
          else if (endpoint === '/api/admin/user-verifications') body = users.filter(item => !url.searchParams.get('status') || item.verificationStatus === url.searchParams.get('status'));
          else if (endpoint === '/api/admin/company-registrations') body = ['PENDING', 'APPROVED', 'REJECTED'].filter(status => !url.searchParams.get('status') || status === url.searchParams.get('status')).map((registrationStatus, index) => ({ ...companies[0], id: index + 1, name: `ثبت ${registrationStatus}`, registrationStatus, createdAt: date }));
          else throw new Error(`Unexpected API request: ${url}`);
          return route.fulfill({ contentType: 'application/json', body: JSON.stringify(body) });
        });
        await page.goto(origin + url);
        await page.locator('main p-table tbody tr').first().waitFor();
        await test(page, requests);
        checks++;
        console.log(`PASS ${url}`);
      } finally { await context.close(); }
    }
    async function select(page, label, value) {
      const combobox = page.getByRole('combobox', { name: label, exact: true });
      await combobox.click();
      await page.getByRole('option', { name: value, exact: true }).click();
      await combobox.filter({ hasText: value }).waitFor();
    }
    async function rows(page, count) {
      await page.waitForFunction(count => document.querySelectorAll('main p-table tbody tr').length === count, count);
      assert.equal(await page.locator('main p-table tbody tr').count(), count);
    }
    for (const [role, url] of [['STUDENT', '/student/opportunity-applications'], ['COMPANY_SUPERVISOR', '/company/opportunities/1/applications']]) {
      await scenario(role, url, async (page, requests) => {
        await rows(page, 3);
        const before = requests.length;
        for (const [status, label] of Object.entries(applicationLabels)) {
          await select(page, 'فیلتر وضعیت درخواست', label);
          await rows(page, 1);
          assert.match(await page.locator('main p-table tbody').innerText(), new RegExp(status));
        }
        await select(page, 'فیلتر وضعیت درخواست', 'همه وضعیت‌ها');
        await rows(page, 3);
        assert.equal(requests.length, before, 'Client filtering must not reload the API');
      });
    }
    await scenario('COMPANY_SUPERVISOR', '/company/opportunities', async page => {
      for (const [status, label] of [['OPEN', 'باز'], ['CLOSED', 'بسته']]) {
        await select(page, 'فیلتر وضعیت فرصت', label);
        await rows(page, 1);
        assert.match(await page.locator('main p-table tbody').innerText(), new RegExp(status));
      }
      await select(page, 'فیلتر وضعیت فرصت', 'همه وضعیت‌ها');
      await rows(page, 2);
    });
    await scenario('UNIVERSITY_SUPERVISOR', '/university/companies', async page => {
      assert.equal(await page.getByRole('button', { name: 'واجد شرایط تأیید', exact: true }).count(), 0);
      await select(page, 'فیلتر وضعیت تأیید شرکت', 'تأیید شده');
      await rows(page, 1);
      assert.match(await page.locator('main p-table tbody').innerText(), /شرکت تأییدشده/);
      await select(page, 'فیلتر وضعیت تأیید شرکت', 'همه شرکت‌ها');
      await rows(page, 2);
      await select(page, 'فیلتر وضعیت تأیید شرکت', 'واجد شرایط تأیید');
      await rows(page, 1);
      assert.match(await page.locator('main p-table tbody').innerText(), /شرکت جدید/);
    });
    for (const [role, url] of [['PROFESSOR', '/professor/internships'], ['COMPANY_SUPERVISOR', '/company/internships?status=CANCELLED']]) {
      await scenario(role, url, async page => {
        await select(page, 'فیلتر وضعیت پرونده', 'لغو شده');
        await rows(page, 1);
        assert.match(await page.locator('main p-table tbody').innerText(), /دانشجو CANCELLED/);
        if (role === 'PROFESSOR') assert.doesNotMatch(await page.locator('main p-table tbody').innerText(), /مدارک ناقص/);
        await select(page, 'فیلتر وضعیت پرونده', 'همه وضعیت‌ها');
        await rows(page, role === 'PROFESSOR' ? 4 : 6);
      });
    }
    await scenario('UNIVERSITY_SUPERVISOR', '/university/applications', async (page, requests) => {
      await select(page, 'فیلتر وضعیت پرونده', 'همه وضعیت‌ها');
      await rows(page, cases.length);
      assert.equal(new URL(page.url()).searchParams.get('status'), 'ALL');
      assert.equal(requests.filter(url => url.pathname === '/api/university/internship-cases').at(-1).searchParams.has('status'), false);
      assert.match(await page.locator('main p-table tbody tr').filter({ hasText: 'دانشجو PENDING_UNIVERSITY_REVIEW' }).innerText(), /اولویت PENDING_UNIVERSITY_REVIEW/);
      await select(page, 'فیلتر وضعیت پرونده', 'لغو شده');
      await rows(page, 1);
      await page.reload();
      await rows(page, 1);
      assert.match(await page.getByRole('combobox', { name: 'فیلتر وضعیت پرونده' }).innerText(), /لغو شده/);
    });
    await scenario('UNIVERSITY_SUPERVISOR', '/university/internship-terms', async page => {
      await select(page, 'فیلتر وضعیت ترم', 'بسته');
      await rows(page, 1);
      assert.equal(await page.getByRole('button', { name: 'ایجاد و باز کردن ترم' }).isDisabled(), true, 'Filtering closed terms must not hide the current open term from business rules');
      await page.getByRole('button', { name: 'پرونده‌های ترم' }).click();
      const dialog = page.getByRole('dialog');
      await dialog.locator('p-table tbody tr').first().waitFor();
      await dialog.locator('.p-paginator-next').click();
      await select(page, 'فیلتر وضعیت پرونده‌های ترم', 'لغو شده');
      await dialog.locator('tbody tr').filter({ hasText: 'دانشجو CANCELLED 12' }).waitFor();
      assert.equal(await dialog.locator('p-table tbody tr').count(), 1);
      await select(page, 'فیلتر وضعیت پرونده‌های ترم', 'مردود');
      await dialog.getByText('پرونده‌ای در این وضعیت وجود ندارد.', { exact: true }).waitFor();
      await select(page, 'فیلتر وضعیت پرونده‌های ترم', 'پیش‌نویس');
      await dialog.locator('tbody tr').filter({ hasText: 'دانشجو DRAFT 13' }).waitFor();
      await select(page, 'فیلتر وضعیت پرونده‌های ترم', 'همه وضعیت‌ها');
      await dialog.locator('tbody tr').filter({ hasText: 'دانشجو ACTIVE 0' }).waitFor();
    });
    await scenario('ADMIN', '/admin/users', async (page, requests) => {
      await page.getByRole('combobox', { name: 'وضعیت حساب', exact: true }).click();
      await page.getByRole('option', { name: 'غیرفعال', exact: true }).click();
      await rows(page, 1);
      assert.match(await page.locator('main p-table tbody').innerText(), /استاد غیرفعال/);
      assert.equal(requests.filter(url => url.pathname === '/api/admin/users').at(-1).searchParams.get('isActive'), 'false');
      await page.getByRole('combobox', { name: 'فیلتر نقش', exact: true }).click();
      await page.getByRole('option', { name: 'دانشجو', exact: true }).click();
      await page.getByText('کاربری با این فیلترها یافت نشد.', { exact: true }).waitFor();
    });
    await scenario('ADMIN', '/admin/user-verifications', async (page, requests) => {
      await page.getByRole('combobox', { name: 'وضعیت احراز هویت', exact: true }).click();
      await page.getByRole('option', { name: 'تأیید شده', exact: true }).click();
      await rows(page, 1);
      await page.locator('main p-table tbody').getByText('استاد غیرفعال', { exact: true }).waitFor();
      assert.equal(requests.filter(url => url.pathname === '/api/admin/user-verifications').at(-1).searchParams.get('status'), 'APPROVED');
    });
    await scenario('ADMIN', '/admin/company-registrations', async page => {
      await select(page, 'فیلتر وضعیت ثبت شرکت', 'همه وضعیت‌ها');
      await rows(page, 3);
    });
    assert.deepEqual(errors, []);
    console.log(`Status filter smoke completed: ${checks} scenarios, no page errors.`);
  } finally {
    if (browser) await browser.close();
    await new Promise(resolve => server.close(resolve));
  }
})().catch(error => { console.error(error); process.exitCode = 1; });
