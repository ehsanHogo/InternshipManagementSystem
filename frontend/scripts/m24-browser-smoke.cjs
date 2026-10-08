// M24 acceptance: run against a dedicated, freshly seeded PostgreSQL database.
// API calls arrange real workflow data; browser checks exercise the production UI.
const { chromium } = require(process.env.PLAYWRIGHT_MODULE || 'playwright');
const assert = require('node:assert/strict');
const base = process.env.SMOKE_BASE_URL || 'http://127.0.0.1:14225';
const stamp = Date.now();
const password = 'Demo123!';
const errors = [];
const pages = {};
let checks = 0;
async function check(name, action) {
 try { await action(); console.log(`PASS ${++checks} ${name}`); }
 catch(error) {
  for(const [role,page] of Object.entries(pages)) {
   await page.screenshot({path:`/tmp/m24-failure-${role}.png`,fullPage:true}).catch(()=>{});
  }
  console.error('Browser page errors:',errors);throw error;
 }
}
async function api(token, method, path, body, want = 200) {
 const headers = token ? { Authorization: `Bearer ${token}` } : {};
 if (body && !(body instanceof FormData)) headers['Content-Type'] = 'application/json';
 const response = await fetch(base + '/api' + path, { method, headers, body: body ? body instanceof FormData ? body : JSON.stringify(body) : undefined });
 const raw = await response.text();
 assert.equal(response.status, want, `${method} ${path}: ${raw}`);
 return raw ? JSON.parse(raw) : null;
}
const auth = (email, secret = password) => api('', 'POST', '/auth/login', { email, password: secret });
const pdf = { name: 'acceptance.pdf', mimeType: 'application/pdf', buffer: Buffer.from('%PDF-1.4\nM24 acceptance\n%%EOF') };
function upload(field) { const body = new FormData(); body.append(field, new Blob([pdf.buffer], {type:pdf.mimeType}), pdf.name); return body; }
async function response(page, path, method, action) {
 const [r] = await Promise.all([page.waitForResponse(r => new URL(r.url()).pathname === '/api'+path && r.request().method() === method), action()]);
 assert(r.ok(), `${method} ${path}: ${await r.text()}`); return r.json();
}
async function login(page, email, secret = password) {
 await page.goto(base+'/login'); await page.locator('#email').fill(email); await page.locator('#password').fill(secret);
 await page.locator('button[type="submit"]').click(); await page.waitForURL(url => url.pathname !== '/login');
 await page.locator('.notification-bell').waitFor();
}
async function text(page, value) { await page.getByText(value, {exact:false}).first().waitFor(); }
async function visit(page, path) {
 await page.goto(base+path); await page.waitForURL(base+path); await page.getByRole('heading',{level:1}).waitFor();
 await page.waitForLoadState('networkidle');
 assert.equal(await page.locator('.error-state').count(),0, `error state at ${path}`);
}
(async () => {
 const browser = await chromium.launch({executablePath:process.env.CHROME_PATH||'/usr/bin/google-chrome',headless:true,args:['--no-sandbox']});
 try {
  const admin=await auth('admin@demo.local'), university=await auth('university@demo.local'), professor=await auth('professor@demo.local');

  async function pageFor(role,email,secret) {
   const context=await browser.newContext(); const page=await context.newPage(); page.on('pageerror',e=>errors.push(e.message));
   await login(page,email,secret); pages[role]=page; return page;
  }
  const a=await pageFor('admin','admin@demo.local');
  const u=await pageFor('university','university@demo.local');
  const p=await pageFor('professor','professor@demo.local');
  const companyEmail=`m24-company-${stamp}@example.test`;
  const companyInput={supervisor:{fullName:'سرپرست پذیرش نهایی',email:companyEmail,password,phone:'09120000000',jobTitle:'مدیر'},
   company:{name:`شرکت پذیرش نهایی ${stamp}`,nationalId:String(stamp),economicCode:'e'+stamp,phone:'02112345678',email:`office-${stamp}@example.test`,address:'تهران'}};
  const registered=await api('','POST','/auth/company-register',companyInput,201);
  const company=await auth(companyEmail), companyID=registered.account.company.id;
  const c=await pageFor('company',companyEmail);
  await check('H: pending Company restricted shell, personal pages and direct routes',async()=>{
   assert.equal(registered.account.company.registrationStatus,'PENDING');assert.equal(registered.account.company.isApproved,false);
   await c.waitForURL('**/company/profile');await text(c,'در انتظار بررسی مدیر سیستم');
   await c.getByRole('button',{name:'باز کردن منو'}).click();assert.equal(await c.getByRole('link',{name:'فرصت‌های کارآموزی'}).count(),0);await c.keyboard.press('Escape');
   await c.goto(base+'/company/opportunities');await c.waitForURL('**/company/profile');
   await visit(c,'/profile');await visit(c,'/notifications');await api(company.token,'GET','/company/opportunities',undefined,403);
  });
  await check('H: rejected Company edits registration without resubmission; explicit resubmit then approval',async()=>{
   await api(admin.token,'POST',`/admin/company-registrations/${companyID}/reject`,{reason:'نشانی شرکت را اصلاح کنید'});
   await visit(c,'/company/profile');await text(c,'نشانی شرکت را اصلاح کنید');
   await c.locator('[formGroupName="company"] [formControlName="address"]').fill('تهران، نشانی اصلاح شده');
   const updated=await response(c,'/company/profile','PUT',()=>c.getByRole('button',{name:'ذخیره اطلاعات شرکت'}).click());
   assert.equal(updated.company.registrationStatus,'REJECTED');
   await response(c,'/company/registration/resubmit','POST',()=>c.getByRole('button',{name:'ارسال مجدد برای بررسی'}).click());
   await api(admin.token,'POST',`/admin/company-registrations/${companyID}/approve`);
   await visit(c,'/company/opportunities');assert.equal((await api(company.token,'GET','/company/profile')).company.isApproved,false);
  });
  await check('all operational roles: navigation, Profile, Notifications, and cross-role direct-route guards',async()=>{
   const paths={admin:['/dashboard','/admin/company-registrations','/admin/user-verifications','/admin/users'],
    university:['/dashboard','/university/students','/university/professors','/university/professor-assignments','/university/internship-terms','/university/applications','/university/companies','/university/company-supervisors'],
    professor:['/dashboard','/professor/internships'],company:['/dashboard','/company/profile','/company/opportunities','/company/internships']};
   for(const [role, routes] of Object.entries(paths)) {
    const page=pages[role];for(const route of [...routes,'/profile','/notifications'])await visit(page,route);
    await page.goto(base+(role==='admin'?'/student/application':'/admin/users'));await page.waitForURL('**/dashboard');
   }
  });
  const opportunities=[];
  for(let i=0;i<4;i++) opportunities.push(await api(company.token,'POST','/company/opportunities',{title:`فرصت پذیرش ${i} ${stamp}`,description:'آزمون گردش کار نهایی',workField:'نرم‌افزار',location:'تهران'},201));
  async function student(label) {
   const created=await api(university.token,'POST','/university/students',{fullName:`دانشجوی پذیرش ${label} ${stamp}`,email:`m24-${label}-${stamp}@example.test`,studentNumber:`${stamp}${label}`,major:'کامپیوتر'},201);
   await api(university.token,'POST','/university/professor-assignments',{studentId:created.user.id,professorId:professor.user.id});
   return {...await auth(created.user.email,created.temporaryPassword), secret:created.temporaryPassword};
  }
  const st=await student('passed');
  const s=await pageFor('student',st.user.email,st.secret);
  await check('Student empty states: no open Term, applications, reports, Final Report, Notifications',async()=>{
   for(const path of ['/dashboard','/student/opportunities','/student/opportunity-applications','/student/application','/student/weekly-reports','/student/final-report','/student/approved-companies','/profile','/notifications'])await visit(s,path);
   await visit(s,'/student/application');await text(s,'در حال حاضر ترم کارآموزی بازی وجود ندارد');
   await api(st.token,'POST','/student/internship-case',undefined,409);
   await s.goto(base+'/admin/users');await s.waitForURL('**/dashboard');
  });
  const term=await api(university.token,'POST','/university/internship-terms',{academicYear:1405,termType:'SUMMER'},201);
  let item;
  await check('DRAFT is created through UI without an accepted Application and Term is automatic',async()=>{
   await visit(s,'/student/application');item=await response(s,'/student/internship-case','POST',()=>s.getByRole('button',{name:'ثبت درخواست رسمی کارآموزی'}).click());
   assert.equal(item.status,'DRAFT');assert.equal(item.termId,term.id);assert.equal(item.preferences.length,0);
   await api(st.token,'PUT','/student/internship-case',{termId:term.id},400);
  });
  const caseID=item.id;
  let first;
  await check('A: Student applies through browser; Company applicant detail and résumé remain usable',async()=>{
   await visit(s,`/student/opportunities/${opportunities[0].id}`);await s.locator('input[type="file"]').setInputFiles(pdf);
   first=await response(s,`/student/opportunities/${opportunities[0].id}/apply`,'POST',()=>s.getByRole('button',{name:'ارسال درخواست'}).click());
   await visit(c,`/company/opportunities/${opportunities[0].id}/applications`);await text(c,st.user.fullName);
   await visit(c,`/company/opportunity-applications/${first.id}`);await text(c,st.user.fullName);
   await api(company.token,'POST',`/company/opportunity-applications/${first.id}/accept`,{});
   await visit(s,`/student/opportunity-applications/${first.id}`);await text(s,'پذیرفته');
  });
  await api(st.token,'PUT','/student/internship-case',{passedCredits:100,mobile:'09120000000'});
  let current=await api(st.token,'PUT','/student/internship-case/preferences',{opportunityApplicationIds:[first.id]});
  await api(st.token,'POST','/student/internship-case/submit');
  await check('B: preference revision permits new applications, replacement, reorder and same-case resubmission',async()=>{
   await api(university.token,'POST',`/university/internship-cases/${caseID}/request-revision`,{comment:'فرصت جایگزین را انتخاب کنید'});
   await visit(s,'/student/application');await text(s,'فرصت جایگزین را انتخاب کنید');
   assert.equal((await api(st.token,'POST','/student/internship-case')).id,caseID);
   const additional=[];
   for(let i=1;i<=2;i++) {
    assert.equal((await api(st.token,'GET',`/student/opportunities/${opportunities[i].id}`)).canApply,true);
    const app=await api(st.token,'POST',`/student/opportunities/${opportunities[i].id}/apply`,upload('resume'),201);
    await api(company.token,'POST',`/company/opportunity-applications/${app.id}/accept`,{});additional.push(app);
   }
   await api(st.token,'PUT','/student/internship-case/preferences',{opportunityApplicationIds:[first.id,...additional.map(x=>x.id)]});
   current=await api(st.token,'PUT','/student/internship-case/preferences',{opportunityApplicationIds:[additional[1].id,additional[0].id]});
   assert.deepEqual(current.preferences.map(x=>x.priority),[1,2]);
   assert.equal(current.id,caseID);assert.equal(current.termId,term.id);
   await api(st.token,'POST','/student/internship-case/submit');
   assert.equal((await api(st.token,'GET',`/student/opportunities/${opportunities[3].id}`)).canApply,false);
   await api(st.token,'POST',`/student/opportunities/${opportunities[3].id}/apply`,upload('resume'),409);
  });
  const details={internshipSubject:'پذیرش نهایی نرم‌افزار',startDate:'2030-01-01',workplaceAddress:'تهران',workplacePhone:'02112345678'};
  await check('C: untrusted operational Company selected; correction loop activates immediately with future StartDate',async()=>{
   await api(university.token,'POST',`/university/internship-cases/${caseID}/approve-placement`,{preferenceId:current.preferences[0].id,letterNumber:'M24',letterDate:'2026-10-08'});
   await visit(c,`/company/internships/${caseID}`);await c.locator('[formControlName="internshipSubject"]').waitFor();
   await api(company.token,'POST',`/company/internship-cases/${caseID}/placement-details`,details);
   await visit(u,`/university/applications/${caseID}`);await text(u,details.internshipSubject);
   await api(university.token,'POST',`/university/internship-cases/${caseID}/request-placement-correction`,{comment:'نشانی را تکمیل کنید'});
   await visit(c,`/company/internships/${caseID}`);await text(c,'نشانی را تکمیل کنید');
   await api(company.token,'POST',`/company/internship-cases/${caseID}/placement-details`,{...details,workplaceAddress:'تهران، نشانی تکمیل شده'});
   const active=await api(university.token,'POST',`/university/internship-cases/${caseID}/final-approve`);
   assert.equal(active.status,'ACTIVE');assert(active.activatedAt);assert.equal(active.selectedPreference.application.opportunity.company.isApproved,false);
   await visit(s,'/student/weekly-reports');await visit(s,'/student/final-report');await text(s,'گزارش نهایی هنوز قابل ارسال نیست');
   await visit(p,`/professor/internships/${caseID}`);await text(p,details.internshipSubject);
  });
  const weekInput=week=>({weekNumber:week,startDate:'2026-10-08',endDate:'2026-10-09',activityDescription:`فعالیت هفته ${week}`});
  await check('D: both reviewers request weekly revision; editing/resubmission resets both approvals',async()=>{
   const week=await api(st.token,'POST','/student/internship-case/weekly-reports',weekInput(1),201);
   const path=`/student/internship-case/weekly-reports/${week.id}`;
   await api(st.token,'POST',path+'/submit');
   for(const role of ['company','professor']) {
    const reviewer=role==='company'?company:professor;
    await api(reviewer.token,'POST',`/${role}/internship-cases/${caseID}/weekly-reports/${week.id}/request-revision`,{comment:'شرح فعالیت را اصلاح کنید'});
    await api(st.token,'PUT',path,{...weekInput(1),activityDescription:'شرح فعالیت اصلاح شده'});
    const resubmitted=await api(st.token,'POST',path+'/submit');
    assert.equal(resubmitted.companyReviewStatus,'PENDING');assert.equal(resubmitted.professorReviewStatus,'PENDING');assert.equal(resubmitted.status,'SUBMITTED');
   }
   for(const role of ['company','professor'])await api((role==='company'?company:professor).token,'POST',`/${role}/internship-cases/${caseID}/weekly-reports/${week.id}/approve`);
   await api(st.token,'POST','/student/internship-case/final-report',upload('file'),409);
  });
  const evaluation=Object.fromEntries(['attendanceRating','participationRating','learningRating','interestRating','persistenceRating','suggestionRating','resourceUsageRating','reportQualityRating','projectPerformanceRating'].map(k=>[k,'GOOD']));
  Object.assign(evaluation,{leaveDays:0,absenceDays:0});
  const laterWeeks=[];
  for(let week=2;week<=8;week++) {
   const report=await api(st.token,'POST','/student/internship-case/weekly-reports',weekInput(week),201);laterWeeks.push(report);
   await api(st.token,'POST',`/student/internship-case/weekly-reports/${report.id}/submit`);
   await api(company.token,'POST',`/company/internship-cases/${caseID}/weekly-reports/${report.id}/approve`);
  }
  await check('Company Evaluation requires Company approvals independently of Professor approvals',async()=>{
   await api(company.token,'POST',`/company/internship-cases/${caseID}/evaluation`,evaluation,201);
   await api(company.token,'POST',`/company/internship-cases/${caseID}/evaluation`,evaluation,409);
   await api(st.token,'POST','/student/internship-case/final-report',upload('file'),409);
   await api(professor.token,'POST',`/professor/internship-cases/${caseID}/complete`,{result:'GOOD'},409);
   for(const report of laterWeeks)await api(professor.token,'POST',`/professor/internship-cases/${caseID}/weekly-reports/${report.id}/approve`);
  });
  let final;
  await check('E: Final Report upload and replacement through browser, approval and finalization readiness',async()=>{
   await visit(s,'/student/final-report');await s.locator('input[type="file"]').setInputFiles(pdf);
   final=await response(s,'/student/internship-case/final-report','POST',()=>s.getByRole('button',{name:'ارسال گزارش نهایی'}).click());
   await api(professor.token,'POST',`/professor/internship-cases/${caseID}/final-report/request-revision`,{comment:'گزارش نهایی را اصلاح کنید'});
   await visit(s,'/student/final-report');await text(s,'گزارش نهایی را اصلاح کنید');await s.locator('input[type="file"]').setInputFiles({...pdf,name:'corrected.pdf'});
   const replacement=await response(s,'/student/internship-case/final-report','POST',()=>s.getByRole('button',{name:'ارسال مجدد گزارش'}).click());
   assert.equal(replacement.id,final.id);assert.equal(replacement.status,'SUBMITTED');assert.notEqual(replacement.currentFileId,final.currentFileId);final=replacement;
   await api(professor.token,'POST',`/professor/internship-cases/${caseID}/final-report/approve`);
   await visit(p,`/professor/internships/${caseID}`);await p.locator('[formControlName="result"]').waitFor();
   assert.equal((await api(professor.token,'GET',`/professor/internship-cases/${caseID}`)).canProfessorComplete,true);
  });
  await check('A: PASSED history, immutable rating through browser, permanent application/case blocks',async()=>{
   await api(professor.token,'POST',`/professor/internship-cases/${caseID}/complete`,{result:'GOOD',comment:'پذیرش نهایی'});
   await visit(s,'/student/application');await text(s,'دوره کارآموزی را با موفقیت گذرانده‌اید');
   await s.getByRole('button',{name:'مشاهده پرونده و گزارش‌ها'}).click();await text(s,'جزئیات پرونده تاریخی');
   assert.equal(await s.locator('app-case-reports details').count(),8);await text(s,'corrected.pdf');
   await s.getByRole('button',{name:'۵ از ۵'}).click();await response(s,`/student/internship-cases/${caseID}/rating`,'POST',()=>s.getByRole('button',{name:'ثبت امتیاز'}).click());
   await text(s,'امتیاز ثبت‌شده نهایی است');
   await api(st.token,'POST',`/student/internship-cases/${caseID}/rating`,{rating:3},409);
   await api(st.token,'POST','/student/internship-case',undefined,409);
   await api(st.token,'POST',`/student/opportunities/${opportunities[3].id}/apply`,upload('resume'),409);
   await api(st.token,'GET','/student/internship-case',undefined,404);
   await api(st.token,'GET','/student/internship-case/weekly-reports',undefined,400);
   await api(st.token,'GET','/student/internship-case/final-report',undefined,404);
  });
  await check('rating is Company-wide; Company DTOs hide all ratings and preference priority; trust remains manual',async()=>{
   const catalog=await api(st.token,'GET','/student/opportunities');
   const own=catalog.filter(x=>x.company.id===companyID);assert.equal(own.length,4);
   for(const opportunity of own){assert.equal(opportunity.companyAverageRating,5);assert.equal(opportunity.companyRatingCount,1);assert(!('nationalId' in opportunity.company));}
   for(const path of ['/company/opportunities','/company/profile',`/company/internship-cases/${caseID}`,`/company/internship-cases/history/${caseID}`,`/company/opportunity-applications/${first.id}`]) {
    const view=await api(company.token,'GET',path);assert(!/companyAverageRating|companyRatingCount|studentRating|"priority"/.test(JSON.stringify(view)),path);
   }
   assert.equal((await api(company.token,'GET','/company/profile')).company.isApproved,false);
   await visit(u,'/university/companies');await text(u,companyInput.company.name);
  });
  await check('personal Notifications render, mark only displayed snapshot read and clear unread badge',async()=>{
   const list=await api(st.token,'GET','/notifications');assert(list.some(n=>n.message.includes('قبول')));
   await visit(s,'/notifications');for(const n of list)await s.locator(`[data-notification-id="${n.id}"]`).waitFor();
   await s.waitForFunction(()=>!document.querySelector('.notification-badge'));
   assert.equal((await api(st.token,'GET','/notifications/unread-count')).count,0);
   assert.equal(await s.getByRole('button',{name:/علامت‌گذاری|خواندن همه/}).count(),0);
  });
  const failed=await student('failed');
  let failedID;
  await check('F: full production HTTP flow finalizes FAILED and permits retry without altering history',async()=>{
   const app=await api(failed.token,'POST',`/student/opportunities/${opportunities[0].id}/apply`,upload('resume'),201);
   await api(company.token,'POST',`/company/opportunity-applications/${app.id}/accept`,{});
   const draft=await api(failed.token,'POST','/student/internship-case',undefined,201);failedID=draft.id;
   await api(failed.token,'PUT','/student/internship-case',{passedCredits:100,mobile:'09120000000'});
   const pref=await api(failed.token,'POST','/student/internship-case/preferences',{priority:1,opportunityApplicationId:app.id},201);
   await api(failed.token,'POST','/student/internship-case/submit');
   await api(university.token,'POST',`/university/internship-cases/${draft.id}/approve-placement`,{preferenceId:pref.id,letterNumber:'M24-failed',letterDate:'2026-10-08'});
   await api(company.token,'POST',`/company/internship-cases/${draft.id}/placement-details`,details);
   await api(university.token,'POST',`/university/internship-cases/${draft.id}/final-approve`);
   for(let week=1;week<=8;week++) {
    const r=await api(failed.token,'POST','/student/internship-case/weekly-reports',weekInput(week),201);
    await api(failed.token,'POST',`/student/internship-case/weekly-reports/${r.id}/submit`);
    for(const role of ['company','professor'])await api((role==='company'?company:professor).token,'POST',`/${role}/internship-cases/${draft.id}/weekly-reports/${r.id}/approve`);
   }
   await api(company.token,'POST',`/company/internship-cases/${draft.id}/evaluation`,evaluation,201);
   await api(failed.token,'POST','/student/internship-case/final-report',upload('file'),201);
   await api(professor.token,'POST',`/professor/internship-cases/${draft.id}/final-report/approve`);
   const result=await api(professor.token,'POST',`/professor/internship-cases/${draft.id}/complete`,{result:'FAILED'});assert.equal(result.internship.status,'FAILED');
   await api(failed.token,'POST',`/student/internship-cases/${draft.id}/rating`,{rating:5},409);
   const before=await api(failed.token,'GET',`/student/internship-cases/history/${draft.id}`);
   const retry=await api(failed.token,'POST','/student/internship-case',undefined,201);assert.notEqual(retry.id,draft.id);assert.equal(retry.termId,term.id);
   assert.deepEqual(await api(failed.token,'GET',`/student/internship-cases/history/${draft.id}`),before);
  });
  await check('G: term closure cancels retry, preserves PASSED/FAILED, sends personal cancellation and assigns later OPEN Term',async()=>{
   await api(university.token,'POST',`/university/internship-terms/${term.id}/close`);
   assert.equal((await api(st.token,'GET',`/student/internship-cases/history/${caseID}`)).status,'PASSED');
   assert.equal((await api(failed.token,'GET',`/student/internship-cases/history/${failedID}`)).status,'FAILED');
   const history=await api(failed.token,'GET','/student/internship-cases/history');assert(history.some(x=>x.status==='CANCELLED'));
   assert((await api(failed.token,'GET','/notifications')).some(n=>n.message.includes('بسته شدن ترم')));
   await api(failed.token,'POST','/student/internship-case',undefined,409);
   const later=await api(university.token,'POST','/university/internship-terms',{academicYear:1406,termType:'FIRST'},201);
   assert.equal((await api(failed.token,'POST','/student/internship-case',undefined,201)).termId,later.id);
  });
  await check('file security, IDOR and disabled JWT/login on the live production API',async()=>{
   const outsider=await student('outsider');
   for(const [token,want] of [[st.token,200],[company.token,200],[professor.token,200],[university.token,200],[outsider.token,403],[admin.token,403]]) {
    const r=await fetch(base+`/api/files/${final.currentFileId}/download`,{headers:{Authorization:`Bearer ${token}`}});assert.equal(r.status,want);
   }
   await api(outsider.token,'GET',`/student/internship-cases/history/${caseID}`,undefined,404);
   await api(outsider.token,'GET',`/student/opportunity-applications/${first.id}`,undefined,404);
   await api(admin.token,'POST',`/admin/users/${outsider.user.id}/disable`);
   await api(outsider.token,'GET','/auth/me',undefined,401);
   await api('','POST','/auth/login',{email:outsider.user.email,password:outsider.secret},401);
  });
  await check('mobile Student history and Notifications render without horizontal overflow',async()=>{
   await s.setViewportSize({width:390,height:844});
   for(const route of ['/student/application','/notifications','/profile']) {
    await visit(s,route);assert(await s.evaluate(()=>document.documentElement.scrollWidth<=window.innerWidth+1),route);
   }
  });
  assert.deepEqual(errors,[]);console.log(`M24 browser acceptance completed: ${checks} checks, no page errors.`);
 } finally { await browser.close(); }
})().catch(error=>{console.error(error);process.exitCode=1});
