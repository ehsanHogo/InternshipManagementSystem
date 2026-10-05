// Run against an isolated, freshly seeded database; creates smoke accounts.
// PLAYWRIGHT_MODULE may point to an existing local Playwright installation.
const { chromium } = require(process.env.PLAYWRIGHT_MODULE || 'playwright');
const assert = require('node:assert/strict');
const base = process.env.SMOKE_BASE_URL || 'http://127.0.0.1:14222';
const stamp = Date.now();
const email = `m22-browser-${stamp}@example.test`;
const password = 'University123!';
const reports = [];
async function check(name, action) { await action(); reports.push(name); console.log(`PASS ${name}`); }
async function login(page, email, password) {
 await page.goto(base + '/login'); await page.locator('#email').fill(email); await page.locator('#password').fill(password);
 await page.locator('button[type="submit"]').click(); await page.waitForURL(url => !url.pathname.endsWith('/login'));
}
async function visibleText(page, text) { await page.getByText(text, { exact:false }).first().waitFor({state:'visible'}); }
async function response(page, path, method, click) {
 const wait = page.waitForResponse(r => new URL(r.url()).pathname === '/api' + path && r.request().method() === method);
 const [r] = await Promise.all([wait, click()]); assert(r.ok(), `${method} ${path}: ${r.status()} ${await r.text()}`); return r.json();
}
async function searchUser(page) {
 await page.locator('#user-search').fill(email);
 await response(page, new URL(page.url()).pathname === '/admin/users' ? '/admin/users' : '/admin/user-verifications', 'GET', () => page.getByRole('button',{name:'جستجو',exact:true}).click());
 await page.locator('tr').filter({hasText:email}).getByRole('button',{name:'مشاهده'}).click();
 await page.getByRole('dialog').getByText(email,{exact:true}).waitFor();
}
(async () => {
 const browser = await chromium.launch({ executablePath:process.env.CHROME_PATH || '/usr/bin/google-chrome', headless:true, args:['--no-sandbox'] });
 const context = await browser.newContext(); const adminContext = await browser.newContext();
 const owner = await context.newPage(); const admin = await adminContext.newPage();
 const errors = [];
 for (const page of [owner,admin]) page.on('pageerror', error => errors.push(error.message));
 try {
  await check('public University registration has fixed role and pending state', async () => {
   await owner.goto(base + '/university-supervisor-register'); await visibleText(owner,'ثبت‌نام مسئول آموزش نیازمند تأیید مدیر سیستم است.');
   assert.equal(await owner.locator('select,p-select').count(),0);
   await owner.locator('#register-name').fill('مسئول آموزش آزمون'); await owner.locator('#register-email').fill(email); await owner.locator('#register-phone').fill('09120000000'); await owner.locator('#register-password').fill(password);
   const result=await response(owner,'/auth/university-supervisor-register','POST',()=>owner.getByRole('button',{name:'ثبت‌نام',exact:true}).click());
   assert.equal(result.user.role,'UNIVERSITY_SUPERVISOR'); assert.equal(result.user.verificationStatus,'PENDING'); assert.equal(result.user.isActive,true);
   await visibleText(owner,'ثبت‌نام انجام شد.');
  });
  let token;
  await check('pending login, restricted navigation, and direct-route guard',async()=>{
   await login(owner,email,password); await owner.waitForURL('**/university-supervisor/verification'); await visibleText(owner,'در انتظار بررسی مدیر سیستم');
   token=await owner.evaluate(()=>localStorage.getItem('internship_access_token'));
   await owner.getByRole('button',{name:'باز کردن منو'}).click();
   assert.equal(await owner.getByRole('link',{name:'ترم‌های کارآموزی',exact:true}).count(),0);
   await owner.keyboard.press('Escape');
   await owner.goto(base+'/university/students'); await owner.waitForURL('**/university-supervisor/verification');
  });
  let id;
  await check('Admin verification search, detail, and rejection',async()=>{
   await login(admin,'admin@demo.local','Demo123!'); await admin.goto(base+'/admin/user-verifications'); await searchUser(admin);
   assert.equal(await admin.getByRole('button',{name:'رد',exact:true}).isDisabled(),true);
   await admin.locator('#rejection-reason').fill('اطلاعات تماس را اصلاح کنید');
   const wait=admin.waitForResponse(r=>r.url().endsWith('/reject')&&r.request().method()==='POST');
   await admin.getByRole('button',{name:'رد',exact:true}).click(); const result=await(await wait).json(); id=result.id; assert.equal(result.verificationStatus,'REJECTED');
  });
  await check('rejection reason and M21 Profile/password while rejected',async()=>{
   await owner.getByRole('button',{name:'به‌روزرسانی وضعیت'}).click(); await visibleText(owner,'اطلاعات تماس را اصلاح کنید');
   await owner.goto(base+'/profile'); await owner.locator('#profile-name').fill('مسئول آموزش اصلاح شده');
   const result=await response(owner,'/profile','PUT',()=>owner.getByRole('button',{name:'ذخیره اطلاعات'}).click());assert.equal(result.verificationStatus,'REJECTED');
   await owner.locator('#current-password').fill(password);await owner.locator('#new-password').fill('Changed123!');await owner.locator('#confirm-password').fill('Changed123!');
   await response(owner,'/profile/change-password','POST',()=>owner.getByRole('button',{name:'تغییر رمز عبور',exact:true}).click());
   await owner.goto(base+'/university-supervisor/verification');await visibleText(owner,'رد شده');
  });
  await check('explicit resubmission and Admin approval',async()=>{
   const result=await response(owner,'/university-supervisor/verification/resubmit','POST',()=>owner.getByRole('button',{name:'ارسال مجدد درخواست'}).click());assert.equal(result.id,id);assert.equal(result.verificationStatus,'PENDING');
   await admin.goto(base+'/admin/user-verifications');await searchUser(admin);
   const approved=await response(admin,`/admin/user-verifications/${id}/approve`,'POST',()=>admin.getByRole('button',{name:'تأیید',exact:true}).click());assert.equal(approved.verificationStatus,'APPROVED');
  });
  await check('approval unlocks University without a new JWT',async()=>{
   await owner.getByRole('button',{name:'به‌روزرسانی وضعیت'}).click();await visibleText(owner,'تأیید شده');
   assert.equal(await owner.evaluate(()=>localStorage.getItem('internship_access_token')),token);
   await owner.getByRole('link',{name:'ورود به امکانات دانشگاه'}).click();await owner.waitForURL('**/university/internship-terms');await visibleText(owner,'ترم‌های کارآموزی');
  });
  const managed={};
  for(const kind of ['professors','students']){
   await check(`University ${kind} provisioning and import controls preserved`,async()=>{
    await owner.goto(base+'/university/'+kind);await owner.getByRole('button',{name:kind==='students'?'افزودن دانشجو':'افزودن استاد'}).click();
    const dialog=owner.getByRole('dialog');await dialog.locator('[formControlName="fullName"]').fill(kind==='students'?`دانشجوی آزمون ${stamp}`:`استاد آزمون ${stamp}`);await dialog.locator('[formControlName="email"]').fill(`${kind}-${stamp}@example.test`);
    if(kind==='students'){await dialog.locator('[formControlName="studentNumber"]').fill(String(stamp));await dialog.locator('[formControlName="major"]').fill('کامپیوتر');}
    managed[kind]=await response(owner,'/university/'+kind,'POST',()=>dialog.getByRole('button',{name:'ایجاد حساب'}).click());assert.equal(managed[kind].user.verificationStatus,'NOT_REQUIRED');
    await owner.getByRole('dialog').getByText('حساب کاربری با موفقیت ایجاد شد.',{exact:true}).waitFor();await owner.keyboard.press('Escape');
    await owner.locator('input[type="file"]').waitFor({state:'visible'});
   });
  }
  await check('Professor assignment remains functional',async()=>{
   await owner.goto(base+'/university/professor-assignments');const row=owner.locator('tr').filter({hasText:`دانشجوی آزمون ${stamp}`});
   await row.locator('p-select').click();await owner.getByRole('option',{name:`استاد آزمون ${stamp}`,exact:true}).click();await response(owner,'/university/professor-assignments','POST',()=>row.getByRole('button',{name:'تخصیص استاد'}).click());
  });
  await check('Admin user list/search/role/status filters, read-only identity, disable',async()=>{
   await admin.goto(base+'/admin/users');await admin.locator('#role-filter').click();await admin.getByRole('option',{name:'مسئول آموزش',exact:true}).click();await admin.locator('#active-filter').click();await admin.getByRole('option',{name:'فعال',exact:true}).click();await searchUser(admin);
   const dialog=admin.getByRole('dialog');assert.equal(await dialog.locator('select,p-select').count(),0);assert.equal(await dialog.getByRole('button',{name:/نقش/}).count(),0);
   await response(admin,`/admin/users/${id}/disable`,'POST',()=>dialog.getByRole('button',{name:'غیرفعال کردن حساب'}).click());await visibleText(dialog,'غیرفعال');
  });
  await check('existing JWT blocked and disabled login displays Persian reason',async()=>{
   await owner.goto(base+'/profile');await owner.waitForURL('**/login');await owner.locator('#email').fill(email);await owner.locator('#password').fill('Changed123!');
   const wait=owner.waitForResponse(r=>r.url().endsWith('/api/auth/login')&&r.request().method()==='POST');await owner.locator('button[type="submit"]').click();assert.equal((await wait).status(),401);await visibleText(owner,'حساب کاربری غیرفعال است.');
  });
  await check('enable preserves verification and restores access',async()=>{
   await response(admin,`/admin/users/${id}/enable`,'POST',()=>admin.getByRole('dialog').getByRole('button',{name:'فعال کردن حساب',exact:true}).click());
   await login(owner,email,'Changed123!');await owner.goto(base+'/university/professors');await visibleText(owner,`استاد آزمون ${stamp}`);
  });
  await check('Professor and Student login remain available after provisioning',async()=>{
   for(const kind of ['professors','students']){const c=await browser.newContext();const p=await c.newPage();await login(p,managed[kind].user.email,managed[kind].temporaryPassword);await p.goto(base+(kind==='professors'?'/professor/internships':'/student/application'));await p.getByRole('heading',{level:1}).waitFor();await c.close();}
  });
  await check('M17 public company registration and separate Admin queue',async()=>{
   const c=await browser.newContext();const p=await c.newPage();p.on('pageerror',e=>errors.push(e.message));await p.goto(base+'/company-register');
   const supervisor=p.locator('[formGroupName="supervisor"]');const company=p.locator('[formGroupName="company"]');
   for(const [key,value] of Object.entries({fullName:'سرپرست شرکت آزمون',email:`company-${stamp}@example.test`,phone:'09120000000',jobTitle:'مدیر'}))await supervisor.locator(`[formControlName="${key}"]`).fill(value);
   await p.locator('#registration-password').fill('Company123!');
   for(const [key,value] of Object.entries({name:`شرکت آزمون ${stamp}`,nationalId:String(stamp),economicCode:'e'+stamp,phone:'02112345678',email:`office-${stamp}@example.test`,address:'تهران'}))await company.locator(`[formControlName="${key}"]`).fill(value);
   await response(p,'/auth/company-register','POST',()=>p.locator('button[type="submit"]').click());await p.waitForURL('**/login');await login(p,`company-${stamp}@example.test`,'Company123!');await p.waitForURL('**/company/profile');await visibleText(p,'در انتظار بررسی مدیر سیستم');
   await admin.goto(base+'/admin/company-registrations');const row=admin.locator('tr').filter({hasText:`شرکت آزمون ${stamp}`});await row.getByRole('button',{name:'مشاهده'}).click();await admin.getByRole('dialog').getByText(`شرکت آزمون ${stamp}`,{exact:true}).waitFor();await c.close();
  });
  await check('disabled Professor historical assignment stays visible and cannot be selected',async()=>{
   await admin.goto(base+'/admin/users');await admin.locator('#user-search').fill(managed.professors.user.email);
   await response(admin,'/admin/users','GET',()=>admin.getByRole('button',{name:'جستجو',exact:true}).click());
   await admin.locator('tr').filter({hasText:managed.professors.user.email}).getByRole('button',{name:'مشاهده'}).click();
   await admin.getByRole('dialog').getByText(managed.professors.user.email,{exact:true}).waitFor();
   await response(admin,`/admin/users/${managed.professors.user.id}/disable`,'POST',()=>admin.getByRole('dialog').getByRole('button',{name:'غیرفعال کردن حساب'}).click());
   await owner.goto(base+'/university/professor-assignments');const row=owner.locator('tr').filter({hasText:`دانشجوی آزمون ${stamp}`});
   await row.getByText('غیرفعال؛ تخصیص قبلی',{exact:false}).waitFor();await row.locator('p-select').click();
   assert.equal(await owner.getByRole('option',{name:`استاد آزمون ${stamp}`,exact:true}).count(),0);await owner.keyboard.press('Escape');
  });
  assert.deepEqual(errors,[]);console.log(`Browser smoke completed: ${reports.length} workflows, no page errors.`);
 } finally { await browser.close(); }
})().catch(error=>{console.error(error);process.exitCode=1});
