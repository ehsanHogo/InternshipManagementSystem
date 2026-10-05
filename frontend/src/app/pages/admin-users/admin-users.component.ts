import { Component, inject, signal } from '@angular/core';
import { ActivatedRoute } from '@angular/router';
import { FormsModule } from '@angular/forms';
import { ButtonModule } from 'primeng/button';
import { InputTextModule } from 'primeng/inputtext';
import { TextareaModule } from 'primeng/textarea';
import { SelectModule } from 'primeng/select';
import { TableModule } from 'primeng/table';
import { DialogModule } from 'primeng/dialog';
import { MessageService } from 'primeng/api';
import { finalize } from 'rxjs';
import { AuthService } from '../../auth/auth.service';
import { User, roleLabels, verificationLabels } from '../../auth/auth.models';
import { AdminUser, UserAccessService } from '../../user-access/user-access.service';
import { JalaliDatePipe } from '../../shared/jalali-date/jalali-date.pipe';
import { registrationStatusLabels } from '../../internship/internship.models';
import { userErrorMessage } from '../../shared/http-error-message';

@Component({
 selector: 'app-admin-users',
 imports: [FormsModule, ButtonModule, InputTextModule, TextareaModule, SelectModule, TableModule, DialogModule, JalaliDatePipe],
 templateUrl: './admin-users.component.html', styleUrl: '../workflow-page.scss'
})
export class AdminUsersComponent {
 private readonly access = inject(UserAccessService);
 private readonly messages = inject(MessageService);
 private readonly auth = inject(AuthService);
 readonly verificationOnly = inject(ActivatedRoute).snapshot.data['verificationOnly'] === true;
 readonly roles = roleLabels;
 readonly labels = verificationLabels;
 readonly companyLabels = registrationStatusLabels;
 readonly users = signal<User[]>([]);
 readonly detail = signal<AdminUser | null>(null);
 readonly loading = signal(false);
 readonly failed = signal(false);
 readonly busy = signal(false);
 readonly detailLoading = signal(false);
 readonly detailFailed = signal(false);
 readonly detailId = signal<number | null>(null);
 readonly visible = signal(false);
 readonly statusOptions = [{ label: 'همه وضعیت‌ها', value: '' }, ...(['PENDING','APPROVED','REJECTED'] as const).map(value => ({ label: verificationLabels[value], value }))];
 readonly roleOptions = [{ label: 'همه نقش‌ها', value: '' }, ...Object.entries(roleLabels).map(([value,label]) => ({ value,label }))];
 readonly activeOptions = [{ label: 'همه حساب‌ها', value: '' }, { label: 'فعال', value: 'true' }, { label: 'غیرفعال', value: 'false' }];
 search = ''; role = ''; active = ''; status = this.verificationOnly ? 'PENDING' : ''; reason = '';
 constructor() { this.load(); }
 load(): void {
  this.loading.set(true); this.failed.set(false);
  const filters: Record<string,string> = { search: this.search };
  if (this.verificationOnly) filters['status'] = this.status;
  else { filters['role'] = this.role; filters['isActive'] = this.active; }
  this.access.list(this.verificationOnly, filters).pipe(finalize(() => this.loading.set(false))).subscribe({
   next: users => this.users.set(users), error: error => { this.failed.set(true); this.showError(error); }
  });
 }
 inspect(id: number): void {
  this.detailId.set(id); this.detail.set(null); this.visible.set(true); this.reason = '';
  this.detailLoading.set(true); this.detailFailed.set(false);
  this.access.detail(id, this.verificationOnly).pipe(finalize(() => this.detailLoading.set(false))).subscribe({
   next: user => this.detail.set(user), error: error => { this.detailFailed.set(true); this.showError(error); }
  });
 }
 review(approve: boolean): void {
  const user = this.detail();
  if (!user || this.busy() || (!approve && !this.reason.trim())) return;
  this.busy.set(true);
  this.access.review(user.id, approve, this.reason).pipe(finalize(() => this.busy.set(false))).subscribe({
   next: result => { this.detail.set(result); this.load(); },
   error: error => { this.showError(error); if (error.status === 409) { this.inspect(user.id); this.load(); } }
  });
 }
 rowLabel(user: User): string { return this.verificationOnly ? this.labels[user.verificationStatus] : this.roles[user.role]; }
 canDisable(user: User): boolean { return user.id !== this.auth.user()?.id; }
 toggleActive(): void {
  const user = this.detail(); if (!user || this.busy()) return;
  this.busy.set(true);
  this.access.setActive(user.id, !user.isActive).pipe(finalize(() => this.busy.set(false))).subscribe({
   next: result => { this.detail.set(result); this.load(); }, error: error => this.showError(error)
  });
 }
 private showError(error: Parameters<typeof userErrorMessage>[0]): void {
  this.messages.add({ severity: 'error', summary: 'خطا', detail: userErrorMessage(error, 'درخواست ناموفق بود.') });
 }
}
