// Run only against an isolated seeded database. Creates future workflow events.
const { chromium } = require(process.env.PLAYWRIGHT_MODULE || 'playwright');
const assert = require('node:assert/strict');
const base = process.env.SMOKE_BASE_URL || 'http://127.0.0.1:14223';
const stamp = Date.now();
const password = 'Demo123!';
const errors = [];
let checks = 0;
async function check(name, action) { await action(); ++checks; console.log(`PASS ${name}`); }
async function api(token, method, path, body, want = 200) {
 const headers = token ? {Authorization: `Bearer ${token}`} : {};
 if (body && !(body instanceof FormData)) headers['Content-Type'] = 'application/json';
 const r = await fetch(base + '/api' + path, {method, headers, body: body ? body instanceof FormData ? body : JSON.stringify(body) : undefined});
 const data = await r.json(); assert.equal(r.status, want, `${method} ${path}: ${JSON.stringify(data)}`); return data;
}
const auth = email => api('', 'POST', '/auth/login', {email, password});
async function login(page, email) {
 await page.goto(base + '/login'); await page.locator('#email').fill(email); await page.locator('#password').fill(password);
 await page.locator('button[type="submit"]').click(); await page.waitForURL(url => url.pathname !== '/login');
 await page.locator('.notification-bell').waitFor();
}
async function notificationsPage(browser, email, expectedMessage, options = {}) {
 const context = await browser.newContext(); const page = await context.newPage(); page.on('pageerror', e => errors.push(e.message));
 await login(page, email);
 if (expectedMessage) await page.locator('.notification-badge').waitFor();
 let acknowledged = [];
 await page.route('**/api/notifications/mark-viewed', async route => {
  acknowledged = route.request().postDataJSON().notificationIds;
  for (const id of acknowledged) assert(await page.locator(`[data-notification-id="${id}"]`).isVisible(), 'acknowledged before render');
  if (options.beforeMark) await options.beforeMark();
  await route.continue();
 });
 await page.locator('.notification-bell').click(); await page.waitForURL('**/notifications');
 await page.getByRole('heading', {name:'اعلان‌ها', exact:true}).waitFor();
 if (expectedMessage) {
  await page.getByText(expectedMessage, {exact:false}).first().waitFor();
  await page.waitForFunction(() => document.querySelectorAll('.notification.unread').length === 0);
  await page.waitForFunction(() => Array.from(document.querySelectorAll('.notification')).every(n => n.textContent.includes('خوانده‌شده')));
  await page.waitForFunction(expected => {
   const badge = document.querySelector('.notification-badge'); return expected ? badge?.textContent.trim() === '۱' : !badge;
  }, !!options.beforeMark);
  assert(acknowledged.length > 0);
 } else await page.getByText('هنوز اعلانی ندارید.').waitFor();
 assert.equal(await page.getByRole('button', {name:/خواندن همه|علامت‌گذاری|خوانده‌شده/}).count(),0);
 if (options.restricted) {
  await page.getByRole('button',{name:'باز کردن منو'}).click();
  assert.equal(await page.getByRole('link',{name:/ترم‌های کارآموزی|فرصت‌های کارآموزی|پرونده‌های دانشجویان/}).count(),0);
  await page.keyboard.press('Escape');
 }
 if (options.capture) {
  await page.screenshot({path:'/tmp/m23-notifications-desktop.png',fullPage:true});
  await page.setViewportSize({width:390,height:844});
  await page.screenshot({path:'/tmp/m23-notifications-mobile.png',fullPage:true});
  assert(await page.locator('.notification-bell').isVisible());
  assert(await page.evaluate(() => document.documentElement.scrollWidth <= window.innerWidth), 'mobile horizontal overflow');
 }
 await context.close();
}
(async()=>{
 const browser=await chromium.launch({executablePath:process.env.CHROME_PATH||'/usr/bin/google-chrome',headless:true,args:['--no-sandbox']});
 try{
  const admin=await auth('admin@demo.local'),university=await auth('university@demo.local'),company=await auth('company@demo.local'),student=await auth('student@demo.local');
  const pendingCompanyEmail=`m23-company-${stamp}@example.test`,pendingUniversityEmail=`m23-university-${stamp}@example.test`;
  const companyRegistration=await api('','POST','/auth/company-register',{
   supervisor:{fullName:'سرپرست آزمون',email:pendingCompanyEmail,password,phone:'09120000000',jobTitle:'مدیر'},
   company:{name:`شرکت اعلان ${stamp}`,nationalId:String(stamp),economicCode:'e'+stamp,email:`office-${stamp}@example.test`,address:'تهران',phone:'02112345678'}
  },201);
  const universityRegistration=await api('','POST','/auth/university-supervisor-register',{fullName:'مسئول آموزش آزمون',email:pendingUniversityEmail,password},201);
  await check('Admin categories cover registration and verification',()=>notificationsPage(browser,'admin@demo.local','ثبت‌نام‌های جدید شرکت‌ها نیازمند بررسی است.'));
  await check('PENDING Company notifications and restricted navigation',()=>notificationsPage(browser,pendingCompanyEmail,null,{restricted:true}));
  await check('PENDING University notifications and restricted navigation',()=>notificationsPage(browser,pendingUniversityEmail,null,{restricted:true}));
  const companyId=companyRegistration.account.company.id, universityId=universityRegistration.user.id;
  await api(admin.token,'POST',`/admin/company-registrations/${companyId}/reject`,{reason:'اصلاح اطلاعات'});
  await api(admin.token,'POST',`/admin/user-verifications/${universityId}/reject`,{reason:'اصلاح اطلاعات'});
  await check('REJECTED Company receives personal result and can view notifications',()=>notificationsPage(browser,pendingCompanyEmail,'ثبت‌نام شرکت شما رد شد.',{restricted:true}));
  await check('REJECTED University receives personal result and can view notifications',()=>notificationsPage(browser,pendingUniversityEmail,'درخواست احراز هویت شما رد شد.',{restricted:true}));
  const opportunity=await api(company.token,'POST','/company/opportunities',{title:'فرصت اعلان',description:'آزمون',workField:'نرم‌افزار',location:'تهران'},201);
  const second=await api(company.token,'POST','/company/opportunities',{title:'فرصت دوم',description:'آزمون',workField:'نرم‌افزار',location:'تهران'},201);
  async function apply(id){const form=new FormData();form.append('resume',new Blob(['%PDF-1.4\n test'],{type:'application/pdf'}),'resume.pdf');return api(student.token,'POST',`/student/opportunities/${id}/apply`,form,201)}
  const application=await apply(opportunity.id),lateApplication=await apply(second.id);
  await check('Company applicant category deduplicates two applications',async()=>{
   const list=await api(company.token,'GET','/notifications');assert.equal(list.filter(n=>n.type==='COMPANY_APPLICATIONS_ACTION_REQUIRED'&&!n.isRead).length,1);
   await notificationsPage(browser,'company@demo.local','درخواست‌های جدیدی برای فرصت‌های کارآموزی شما ثبت شده است و نیاز به بررسی دارد.');
  });
  await api(company.token,'POST',`/company/opportunity-applications/${application.id}/accept`,{});
  await check('Student renders personal result, marks exact snapshot, preserves late event badge',()=>notificationsPage(browser,'student@demo.local','درخواست شما توسط شرکت پذیرفته شد.',{
   beforeMark:()=>api(company.token,'POST',`/company/opportunity-applications/${lateApplication.id}/accept`,{}),capture:true
  }));
  await check('Reopening consumes the later Student notification',()=>notificationsPage(browser,'student@demo.local','درخواست شما توسط شرکت پذیرفته شد.'));
  await api(university.token,'POST','/university/internship-terms',{academicYear:1405,termType:'SUMMER'},201);
  const item=await api(student.token,'POST','/student/internship-case',undefined,201);
  await api(student.token,'PUT','/student/internship-case',{passedCredits:100,mobile:'09120000000'});
  const pref=await api(student.token,'POST','/student/internship-case/preferences',{priority:1,opportunityApplicationId:application.id},201);
  await api(student.token,'POST','/student/internship-case/submit');
  await check('University initial case-review category',()=>notificationsPage(browser,'university@demo.local','درخواست‌های کارآموزی جدیدی برای بررسی و تعیین محل کارآموزی وجود دارد.'));
  await api(university.token,'POST',`/university/internship-cases/${item.id}/approve-placement`,{preferenceId:pref.id,letterNumber:'m23',letterDate:'2026-10-08'});
  await api(company.token,'POST',`/company/internship-cases/${item.id}/placement-details`,{internshipSubject:'آزمون',startDate:'2026-10-08',workplaceAddress:'تهران',workplacePhone:'02112345678'});
  await api(university.token,'POST',`/university/internship-cases/${item.id}/final-approve`);
  const report=await api(student.token,'POST','/student/internship-case/weekly-reports',{weekNumber:1,startDate:'2026-10-08',endDate:'2026-10-09',activityDescription:'گزارش آزمون'},201);
  await api(student.token,'POST',`/student/internship-case/weekly-reports/${report.id}/submit`);
  await check('Professor general reports alert and queue navigation',()=>notificationsPage(browser,'professor@demo.local','گزارش‌های جدیدی برای بررسی شما وجود دارد.'));
  await check('Disabled account cannot enter Notifications with existing JWT',async()=>{
   const context=await browser.newContext(),page=await context.newPage();page.on('pageerror',e=>errors.push(e.message));
   await login(page,pendingUniversityEmail);
   await api(admin.token,'POST',`/admin/users/${universityId}/disable`);
   await page.goto(base+'/notifications');await page.waitForURL('**/login');
   await context.close();
  });
  assert.deepEqual(errors,[]);console.log(`Browser smoke completed: ${checks} checks, no page errors.`);
 }finally{await browser.close()}
})().catch(e=>{console.error(e);process.exitCode=1});
