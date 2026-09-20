import { HttpErrorResponse } from '@angular/common/http';
import { Component, inject, signal } from '@angular/core';
import { FormBuilder, ReactiveFormsModule, Validators } from '@angular/forms';
import { ActivatedRoute } from '@angular/router';
import { finalize } from 'rxjs';
import { MessageService } from 'primeng/api';
import { ButtonModule } from 'primeng/button';
import { DialogModule } from 'primeng/dialog';
import { InputTextModule } from 'primeng/inputtext';
import { TableModule } from 'primeng/table';
import { TagModule } from 'primeng/tag';

import { User } from '../../auth/auth.models';
import { CreatedAccount, ImportResult, ImportRowResult } from '../../university-management/university-management.models';
import { UniversityManagementService } from '../../university-management/university-management.service';

type ManagedKind = 'students' | 'professors';

@Component({
  selector: 'app-university-users',
  imports: [ReactiveFormsModule, ButtonModule, DialogModule, InputTextModule, TableModule, TagModule],
  templateUrl: './university-users.component.html',
  styleUrl: '../workflow-page.scss'
})
export class UniversityUsersComponent {
  private readonly management = inject(UniversityManagementService);
  private readonly messages = inject(MessageService);
  private readonly formBuilder = inject(FormBuilder);
  private readonly route = inject(ActivatedRoute);

  readonly kind = this.route.snapshot.data['kind'] as ManagedKind;
  readonly isStudent = this.kind === 'students';
  readonly title = this.isStudent ? 'دانشجویان' : 'اساتید';
  readonly createLabel = this.isStudent ? 'افزودن دانشجو' : 'افزودن استاد';
  readonly expectedColumns = this.isStudent
    ? ['نام و نام خانوادگی', 'ایمیل', 'شماره دانشجویی', 'رشته']
    : ['نام و نام خانوادگی', 'ایمیل'];
  readonly users = signal<User[]>([]);
  readonly loading = signal(true);
  readonly createDialogVisible = signal(false);
  readonly credentials = signal<CreatedAccount | null>(null);
  readonly importResult = signal<ImportResult | null>(null);
  readonly selectedFile = signal<File | null>(null);
  submitting = false;
  importing = false;

  readonly userForm = this.formBuilder.nonNullable.group({
    fullName: ['', Validators.required],
    email: ['', [Validators.required, Validators.email]],
    studentNumber: [''],
    major: ['']
  });

  constructor() {
    if (this.isStudent) {
      this.userForm.controls.studentNumber.addValidators(Validators.required);
      this.userForm.controls.major.addValidators(Validators.required);
    }
    this.loadUsers();
  }

  openCreateDialog(): void {
    this.userForm.reset();
    this.createDialogVisible.set(true);
  }

  submit(): void {
    if (this.userForm.invalid) {
      this.userForm.markAllAsTouched();
      return;
    }
    this.submitting = true;
    const raw = this.userForm.getRawValue();
    const request = this.isStudent
      ? this.management.createStudent({
          fullName: raw.fullName.trim(), email: raw.email.trim(),
          studentNumber: raw.studentNumber.trim(), major: raw.major.trim()
        })
      : this.management.createProfessor({ fullName: raw.fullName.trim(), email: raw.email.trim() });
    request.pipe(finalize(() => (this.submitting = false))).subscribe({
      next: (account) => {
        this.createDialogVisible.set(false);
        this.credentials.set(account);
        this.loadUsers();
      },
      error: (error: HttpErrorResponse) => this.showError(error, 'ایجاد حساب ناموفق بود.')
    });
  }

  fileChanged(event: Event): void {
    const input = event.target as HTMLInputElement;
    this.selectedFile.set(input.files?.[0] ?? null);
    this.importResult.set(null);
  }

  importFile(): void {
    const file = this.selectedFile();
    if (!file) return;
    this.importing = true;
    const request = this.isStudent ? this.management.importStudents(file) : this.management.importProfessors(file);
    request.pipe(finalize(() => (this.importing = false))).subscribe({
      next: (result) => {
        this.importResult.set(result);
        this.loadUsers();
        this.messages.add({ severity: 'success', summary: 'ورود فایل انجام شد', detail: `${result.created} حساب ایجاد و ${result.skipped} ردیف رد شد.` });
      },
      error: (error: HttpErrorResponse) => this.showError(error, 'ورود فایل اکسل ناموفق بود.')
    });
  }

  copyCredentials(account: CreatedAccount): void {
    const value = `نام کاربری: ${account.user.email}\nرمز عبور موقت: ${account.temporaryPassword}`;
    void navigator.clipboard.writeText(value).then(() =>
      this.messages.add({ severity: 'success', summary: 'کپی شد', detail: 'اطلاعات ورود در کلیپ‌بورد قرار گرفت.' })
    );
  }

  closeCredentials(visible: boolean): void {
    if (!visible) this.credentials.set(null);
  }

  copyImportRow(email: string | undefined, password: string | undefined): void {
    if (!email || !password) return;
    void navigator.clipboard.writeText(`نام کاربری: ${email}\nرمز عبور موقت: ${password}`);
  }

  importStatusLabel(status: ImportRowResult['status']): string {
    return { CREATED: 'ایجاد شد', SKIPPED: 'رد شد', FAILED: 'ناموفق' }[status];
  }

  importStatusSeverity(status: ImportRowResult['status']): 'success' | 'warn' | 'danger' {
    return { CREATED: 'success', SKIPPED: 'warn', FAILED: 'danger' }[status] as 'success' | 'warn' | 'danger';
  }

  private loadUsers(): void {
    this.loading.set(true);
    const request = this.isStudent ? this.management.listStudents() : this.management.listProfessors();
    request.subscribe({
      next: (users) => {
        this.users.set(users);
        this.loading.set(false);
      },
      error: (error: HttpErrorResponse) => {
        this.loading.set(false);
        this.showError(error, 'دریافت فهرست کاربران ناموفق بود.');
      }
    });
  }

  private showError(error: HttpErrorResponse, fallback: string): void {
    const detail = error.status === 0 ? 'ارتباط با سرور برقرار نشد.' : (error.error?.error || fallback);
    this.messages.add({ severity: 'error', summary: 'خطا', detail });
  }
}
