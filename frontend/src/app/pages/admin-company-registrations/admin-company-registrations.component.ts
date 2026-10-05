import { HttpErrorResponse } from '@angular/common/http';
import { Component, inject, signal } from '@angular/core';
import { FormsModule } from '@angular/forms';
import { finalize } from 'rxjs';
import { MessageService } from 'primeng/api';
import { ButtonModule } from 'primeng/button';
import { TableModule } from 'primeng/table';
import { SelectModule } from 'primeng/select';
import { DialogModule } from 'primeng/dialog';
import { TagModule } from 'primeng/tag';
import { TextareaModule } from 'primeng/textarea';
import { CompanyRegistrationDetail, CompanyRegistrationService } from '../../company/company-registration.service';
import { Company, CompanyRegistrationStatus, registrationStatusLabels } from '../../internship/internship.models';
import { JalaliDatePipe } from '../../shared/jalali-date/jalali-date.pipe';
import { userErrorMessage } from '../../shared/http-error-message';

@Component({
 selector: 'app-admin-company-registrations',
 imports: [FormsModule, ButtonModule, TableModule, SelectModule, DialogModule, TagModule, TextareaModule, JalaliDatePipe],
 templateUrl: './admin-company-registrations.component.html',
 styleUrl: '../workflow-page.scss'
})
export class AdminCompanyRegistrationsComponent {
 private readonly registrations = inject(CompanyRegistrationService);
 private readonly messages = inject(MessageService);
 readonly companies = signal<Company[]>([]);
 readonly detail = signal<CompanyRegistrationDetail | null>(null);
 readonly loading = signal(false);
 readonly failed = signal(false);
 readonly detailLoading = signal(false);
 readonly detailFailed = signal(false);
 readonly detailId = signal<number | null>(null);
 readonly visible = signal(false);
 readonly busy = signal(false);
 readonly labels = registrationStatusLabels;
 readonly filters = [{ label: 'همه وضعیت‌ها', value: '' }, ...(['PENDING', 'APPROVED', 'REJECTED'] as const).map(value => ({ label: registrationStatusLabels[value], value }))];
 statusLabel(status: CompanyRegistrationStatus): string { return this.labels[status]; }
 status: CompanyRegistrationStatus | '' = 'PENDING';
 reason = '';
 constructor() { this.load(); }
 load(): void {
  this.loading.set(true); this.failed.set(false);
  this.registrations.list(this.status).pipe(finalize(() => this.loading.set(false))).subscribe({
   next: companies => this.companies.set(companies),
   error: (error: HttpErrorResponse) => { this.failed.set(true); this.showError(error); }
  });
 }
 inspect(id: number): void {
  this.detailId.set(id); this.detail.set(null); this.visible.set(true); this.reason = '';
  this.detailLoading.set(true); this.detailFailed.set(false);
  this.registrations.detail(id).pipe(finalize(() => this.detailLoading.set(false))).subscribe({
   next: detail => this.detail.set(detail),
   error: (error: HttpErrorResponse) => { this.detailFailed.set(true); this.showError(error); }
  });
 }
 review(approve: boolean): void {
  const detail = this.detail();
  if (!detail || this.busy() || (!approve && !this.reason.trim())) return;
  this.busy.set(true);
  const request = approve ? this.registrations.approve(detail.company.id) : this.registrations.reject(detail.company.id, this.reason.trim());
  request.pipe(finalize(() => this.busy.set(false))).subscribe({
   next: () => { this.visible.set(false); this.load(); this.messages.add({ severity: 'success', summary: 'ثبت شد', detail: 'نتیجه بررسی ثبت شرکت ذخیره شد.' }); },
   error: (error: HttpErrorResponse) => { this.showError(error); if (error.status === 409) { this.inspect(detail.company.id); this.load(); } }
  });
 }
 private showError(error: HttpErrorResponse): void {
  this.messages.add({ severity: 'error', summary: 'خطا', detail: userErrorMessage(error, 'درخواست بررسی ثبت شرکت ناموفق بود.') });
 }
}
