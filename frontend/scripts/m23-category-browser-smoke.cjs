// Focused M23 polish checks. Use a separate, freshly seeded database.
const { chromium } = require(process.env.PLAYWRIGHT_MODULE || 'playwright');
const assert = require('node:assert/strict');
const base = process.env.SMOKE_BASE_URL || 'http://127.0.0.1:14223';
const stamp = Date.now();
const password = 'Demo123!';
let checks = 0;
const errors = [];
async function check(name, fn) { await fn(); ++checks; console.log(`PASS ${name}`); }
async function api(token, method, path, body, want = 200) {
 const headers = token ? { Authorization: `Bearer ${token}` } : {};
 if (body && !(body instanceof FormData)) headers['Content-Type'] = 'application/json';
 const response = await fetch(base + '/api' + path, { method, headers, body: body ? body instanceof FormData ? body : JSON.stringify(body) : undefined });
 const result = await response.json(); assert.equal(response.status, want, `${method} ${path}: ${JSON.stringify(result)}`); return result;
}
const auth = (email, secret = password) => api('', 'POST', '/auth/login', { email, password: secret });
async function login(page, email) {
 await page.goto(base + '/login'); await page.locator('#email').fill(email); await page.locator('#password').fill(password);
 await page.locator('button[type="submit"]').click(); await page.waitForURL(url => url.pathname !== '/login'); await page.locator('.notification-bell').waitFor();
}
async function openNotifications(page, token, categories, badge) {
 if (badge) await page.locator('.notification-badge').getByText(badge, {exact:true}).waitFor();
 const snapshot = await api(token, 'GET', '/notifications');
 for (const category of categories) assert(snapshot.some(n => n.type === category), `missing category ${category}`);
 await page.locator('.notification-bell').click(); await page.waitForURL('**/notifications');
 for (const n of snapshot) await page.locator(`[data-notification-id="${n.id}"]`).waitFor();
 await page.waitForFunction(() => !document.querySelector('.notification-badge'));
 assert.equal(await page.locator('.notification.unread').count(), 0);
 assert.equal(await page.getByRole('button', {name:/خواندن همه|علامت‌گذاری|خوانده‌شده/}).count(), 0);
 assert.equal((await api(token,'GET','/notifications/unread-count')).count,0);
 return snapshot;
}
async function follow(page, list, category, title, message, path, verify) {
 const item = list.find(n => n.type === category); assert(item, `missing ${category}`); assert.equal(item.actionPath,path);
 await page.goto(base+'/notifications');
 const card = page.locator(`[data-notification-id="${item.id}"]`);
 await card.getByRole('heading',{name:title,exact:true}).waitFor(); await card.getByText(message,{exact:true}).waitFor();
 await card.getByRole('link',{name:'مشاهده بخش مربوط'}).click(); await page.waitForURL(base+path);
 if (verify) await verify();
}
(async()=>{
 const browser=await chromium.launch({executablePath:process.env.CHROME_PATH||'/usr/bin/google-chrome',headless:true,args:['--no-sandbox']});
 try {
  const admin=await auth('admin@demo.local'), university=await auth('university@demo.local'), company=await auth('company@demo.local'), professor=await auth('professor@demo.local');
  const students=[await auth('student@demo.local')];
  for(let i=1;i<=3;i++) {
   const created=await api(university.token,'POST','/university/students',{fullName:`دانشجوی اعلان ${i} ${stamp}`,email:`m23-category-${i}-${stamp}@example.test`,studentNumber:`${stamp}${i}`,major:'کامپیوتر'},201);
   await api(university.token,'POST','/university/professor-assignments',{studentId:created.user.id,professorId:professor.user.id});
   students.push(await auth(created.user.email,created.temporaryPassword));
  }
  const opportunity=await api(company.token,'POST','/company/opportunities',{title:`فرصت دسته‌بندی ${stamp}`,description:'آزمون اعلان',workField:'نرم‌افزار',location:'تهران'},201);
  const applications=[];
  for(let i=0;i<students.length;i++) {
   const form=new FormData();form.append('resume',new Blob(['%PDF-1.4\n test'],{type:'application/pdf'}),'resume.pdf');
   const application=await api(students[i].token,'POST',`/student/opportunities/${opportunity.id}/apply`,form,201); applications.push(application);
   if(i<3) await api(company.token,'POST',`/company/opportunity-applications/${application.id}/accept`,{});
  }
  await api(university.token,'POST','/university/internship-terms',{academicYear:1405,termType:'SUMMER'},201);
  const cases=[],preferences=[];
  for(let i=0;i<3;i++) {
   const student=students[i]; cases.push(await api(student.token,'POST','/student/internship-case',undefined,201));
   await api(student.token,'PUT','/student/internship-case',{passedCredits:100,mobile:'09120000000'});
   preferences.push(await api(student.token,'POST','/student/internship-case/preferences',{priority:1,opportunityApplicationId:applications[i].id},201));
   await api(student.token,'POST','/student/internship-case/submit');
   if(i<2) await api(university.token,'POST',`/university/internship-cases/${cases[i].id}/approve-placement`,{preferenceId:preferences[i].id,letterNumber:'m23-polish',letterDate:'2026-10-08'});
  }
  await api(company.token,'POST',`/company/internship-cases/${cases[0].id}/placement-details`,{internshipSubject:'نرم‌افزار',startDate:'2026-10-08',workplaceAddress:'تهران',workplacePhone:'02112345678'});
  await api('','POST','/auth/company-register',{
   supervisor:{fullName:'سرپرست آزمون',email:`m23-category-company-${stamp}@example.test`,password,phone:'09120000000',jobTitle:'مدیر'},
   company:{name:`شرکت دسته‌بندی ${stamp}`,nationalId:String(stamp),economicCode:'e'+stamp,email:`office-${stamp}@example.test`,address:'تهران',phone:'02112345678'}
  },201);
  await api('','POST','/auth/university-supervisor-register',{fullName:'مسئول آموزش آزمون',email:`m23-category-university-${stamp}@example.test`,password},201);
  const context=await browser.newContext(),page=await context.newPage(); page.on('pageerror',e=>errors.push(e.message));
  await login(page,'university@demo.local');
  let list;
  await check('University categories coexist; automatic view clears both and badge',async()=>{
   list=await openNotifications(page,university.token,['UNIVERSITY_CASE_REVIEW_ACTION_REQUIRED','UNIVERSITY_PLACEMENT_REVIEW_ACTION_REQUIRED'],'۲');
   assert.equal(list.length,2);
  });
  await check('Initial Case notification opens filtered pending University queue',()=>follow(page,list,'UNIVERSITY_CASE_REVIEW_ACTION_REQUIRED',
   'درخواست کارآموزی جدید','درخواست‌های کارآموزی جدیدی برای بررسی و تعیین محل کارآموزی وجود دارد.','/university/applications?status=PENDING_UNIVERSITY_REVIEW',async()=>{
    await page.locator('.filter-row p-select').getByText('در انتظار بررسی آموزش',{exact:true}).waitFor();
    await page.locator('tr').filter({hasText:students[2].user.fullName}).waitFor();
   }));
  await check('Final placement notification opens filtered final-approval queue',()=>follow(page,list,'UNIVERSITY_PLACEMENT_REVIEW_ACTION_REQUIRED',
   'نیاز به تأیید نهایی','اطلاعات محل کارآموزی پرونده‌های جدیدی ثبت شده و نیازمند بررسی نهایی است.','/university/applications?status=PENDING_FINAL_APPROVAL',async()=>{
    await page.locator('.filter-row p-select').getByText('در انتظار تأیید نهایی آموزش',{exact:true}).waitFor();
    await page.locator('tr').filter({hasText:students[0].user.fullName}).waitFor();
   }));
  await api(university.token,'POST',`/university/internship-cases/${cases[0].id}/final-approve`);
  for(let week=1;week<=2;week++) {
   const report=await api(students[0].token,'POST','/student/internship-case/weekly-reports',{weekNumber:week,startDate:'2026-10-08',endDate:'2026-10-09',activityDescription:'گزارش آزمون دسته‌بندی'},201);
   await api(students[0].token,'POST',`/student/internship-case/weekly-reports/${report.id}/submit`);
  }
  await context.close();
  const companyContext=await browser.newContext(),companyPage=await companyContext.newPage();companyPage.on('pageerror',e=>errors.push(e.message));
  await login(companyPage,'company@demo.local');
  await check('Company applicant, placement and report categories coexist and deduplicate',async()=>{
   list=await openNotifications(companyPage,company.token,['COMPANY_APPLICATIONS_ACTION_REQUIRED','COMPANY_PLACEMENT_ACTION_REQUIRED','COMPANY_WEEKLY_REPORTS_ACTION_REQUIRED'],'۳');
   assert.equal(list.length,3);
  });
  await check('Applicant notification opens existing opportunities and applicant review',()=>follow(companyPage,list,'COMPANY_APPLICATIONS_ACTION_REQUIRED',
   'درخواست جدید کارآموزی','درخواست‌های جدیدی برای فرصت‌های کارآموزی شما ثبت شده است و نیاز به بررسی دارد.','/company/opportunities',async()=>{
    await companyPage.locator('tr').filter({hasText:opportunity.title}).getByRole('button',{name:'متقاضیان'}).click();
    await companyPage.waitForURL(`**/company/opportunities/${opportunity.id}/applications`);
    await companyPage.locator('tr').filter({hasText:students[3].user.fullName}).waitFor();
   }));
  await check('Placement notification opens existing details queue and form',()=>follow(companyPage,list,'COMPANY_PLACEMENT_ACTION_REQUIRED',
   'تکمیل اطلاعات کارآموزی','پرونده‌های کارآموزی جدیدی نیازمند تکمیل اطلاعات محل کارآموزی هستند.','/company/internships?status=PENDING_COMPANY_DETAILS',async()=>{
    await companyPage.locator('.filter-row p-select').getByText('در انتظار ثبت/اصلاح اطلاعات شرکت',{exact:true}).waitFor();
    await companyPage.locator('tr').filter({hasText:students[1].user.fullName}).getByRole('button',{name:'مشاهده و تکمیل اطلاعات'}).click();
    await companyPage.locator('[formControlName="internshipSubject"]').waitFor();
   }));
  await check('Weekly notification opens active cases and actual report review controls',()=>follow(companyPage,list,'COMPANY_WEEKLY_REPORTS_ACTION_REQUIRED',
   'گزارش هفتگی جدید','گزارش‌های هفتگی جدیدی برای بررسی و تأیید شما وجود دارد.','/company/internships?status=ACTIVE',async()=>{
    await companyPage.locator('.filter-row p-select').getByText('در حال انجام کارآموزی',{exact:true}).waitFor();
    await companyPage.locator('tr').filter({hasText:students[0].user.fullName}).getByRole('button',{name:'مشاهده'}).click();
    await companyPage.getByRole('button',{name:'بررسی گزارش'}).first().waitFor();
   }));
  await check('Invalid queue status safely falls back to existing default',async()=>{
   await companyPage.goto(base+'/company/internships?status=UNKNOWN');
   await companyPage.locator('.filter-row p-select').getByText('در انتظار ثبت/اصلاح اطلاعات شرکت',{exact:true}).waitFor();
  });
  await companyContext.close();
  const adminContext=await browser.newContext(),adminPage=await adminContext.newPage();adminPage.on('pageerror',e=>errors.push(e.message));
  await login(adminPage,'admin@demo.local');
  await check('Both Admin categories coexist and become read on view',async()=>{
   list=await openNotifications(adminPage,admin.token,['ADMIN_COMPANY_REGISTRATION_ACTION_REQUIRED','ADMIN_UNIVERSITY_VERIFICATION_ACTION_REQUIRED'],'۲');assert.equal(list.length,2);
  });
  await check('Admin Company category opens registration queue',()=>follow(adminPage,list,'ADMIN_COMPANY_REGISTRATION_ACTION_REQUIRED',
   'ثبت شرکت جدید','ثبت‌نام‌های جدید شرکت‌ها نیازمند بررسی است.','/admin/company-registrations',()=>adminPage.getByRole('heading',{name:'بررسی ثبت شرکت‌ها',exact:true}).waitFor()));
  await check('Admin verification category opens University verification queue',()=>follow(adminPage,list,'ADMIN_UNIVERSITY_VERIFICATION_ACTION_REQUIRED',
   'درخواست احراز هویت جدید','درخواست‌های جدید احراز هویت مسئول آموزش نیازمند بررسی است.','/admin/user-verifications',()=>adminPage.getByRole('heading',{name:'احراز هویت مسئولان آموزش',exact:true}).waitFor()));
  await adminContext.close();
  assert.deepEqual(errors,[]); console.log(`Category browser smoke completed: ${checks} checks, no page errors.`);
 } finally {await browser.close()}
})().catch(error=>{console.error(error);process.exitCode=1});
