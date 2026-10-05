import { HttpErrorResponse } from '@angular/common/http';
import { Component, DestroyRef, inject, signal } from '@angular/core';
import { takeUntilDestroyed } from '@angular/core/rxjs-interop';
import { AbstractControl, FormBuilder, ReactiveFormsModule, ValidationErrors, Validators } from '@angular/forms';
import { finalize } from 'rxjs';
import { MessageService } from 'primeng/api';
import { ButtonModule } from 'primeng/button';
import { CardModule } from 'primeng/card';
import { InputTextModule } from 'primeng/inputtext';
import { RouterLink } from '@angular/router';

import { User, UserRole } from '../../auth/auth.models';
import { AuthService } from '../../auth/auth.service';
import { ProfileService, ProfileUpdate } from '../../profile/profile.service';
import { userErrorMessage } from '../../shared/http-error-message';

const nonblank = (control: AbstractControl): ValidationErrors | null =>
  typeof control.value === 'string' && control.value.trim() ? null : { nonblank: true };

const passwordRules = (control: AbstractControl): ValidationErrors | null =>
  nonblank(control) || (new TextEncoder().encode(control.value).length > 72 ? { passwordLength: true } : null);

const passwordsMatch = (control: AbstractControl): ValidationErrors | null =>
  control.get('newPassword')?.value === control.get('confirmNewPassword')?.value ? null : { passwordMismatch: true };

@Component({
  selector: 'app-profile',
  imports: [ReactiveFormsModule, ButtonModule, CardModule, InputTextModule, RouterLink],
  templateUrl: './profile.component.html',
  styleUrl: './profile.component.scss'
})
export class ProfileComponent {
  private readonly auth = inject(AuthService);
  private readonly profiles = inject(ProfileService);
  private readonly messages = inject(MessageService);
  private readonly fb = inject(FormBuilder);
  private readonly destroyRef = inject(DestroyRef);
  readonly user = this.auth.user;
  readonly loading = signal(true);
  readonly loadFailed = signal(false);
  readonly saving = signal(false);
  readonly changingPassword = signal(false);
  readonly roleLabels: Record<UserRole, string> = {
    STUDENT: 'دانشجو', COMPANY_SUPERVISOR: 'سرپرست شرکت', UNIVERSITY_SUPERVISOR: 'مسئول آموزش',
    PROFESSOR: 'استاد', ADMIN: 'مدیر سامانه'
  };
  readonly form = this.fb.nonNullable.group({
    fullName: ['', [nonblank, Validators.maxLength(200)]],
    phone: ['', Validators.maxLength(50)],
    jobTitle: ['', Validators.maxLength(200)]
  });
  readonly passwordForm = this.fb.nonNullable.group({
    currentPassword: ['', Validators.required],
    newPassword: ['', passwordRules],
    confirmNewPassword: ['', Validators.required]
  }, { validators: passwordsMatch });

  constructor() { this.load(); }

  load(): void {
    this.loading.set(true);
    this.loadFailed.set(false);
    this.auth.refreshUser().pipe(takeUntilDestroyed(this.destroyRef), finalize(() => this.loading.set(false))).subscribe({
      next: user => this.setForm(user),
      error: (error: HttpErrorResponse) => { this.loadFailed.set(true); this.showError(error); }
    });
  }

  private setForm(user: User): void {
    this.form.reset({ fullName: user.fullName, phone: user.phone ?? '', jobTitle: user.jobTitle ?? '' });
  }

  save(): void {
    if (this.saving() || this.loading() || this.loadFailed()) return;
    if (this.form.invalid) { this.form.markAllAsTouched(); return; }
    const values = this.form.getRawValue();
    const input: ProfileUpdate = { fullName: values.fullName, phone: values.phone };
    if (this.user()?.role === 'COMPANY_SUPERVISOR') input.jobTitle = values.jobTitle;
    this.saving.set(true);
    this.profiles.update(input).pipe(takeUntilDestroyed(this.destroyRef), finalize(() => this.saving.set(false))).subscribe({
      next: user => {
        this.setForm(user);
        this.messages.add({ severity: 'success', summary: 'ذخیره شد', detail: 'اطلاعات حساب کاربری به‌روز شد.' });
      },
      error: (error: HttpErrorResponse) => this.showError(error)
    });
  }

  changePassword(): void {
    if (this.changingPassword() || this.loading() || this.loadFailed()) return;
    if (this.passwordForm.invalid) { this.passwordForm.markAllAsTouched(); return; }
    const { currentPassword, newPassword } = this.passwordForm.getRawValue();
    this.changingPassword.set(true);
    this.profiles.changePassword(currentPassword, newPassword)
      .pipe(takeUntilDestroyed(this.destroyRef), finalize(() => this.changingPassword.set(false))).subscribe({
        next: response => {
          this.passwordForm.reset();
          this.messages.add({ severity: 'success', summary: 'رمز عبور تغییر کرد', detail: response.message });
        },
        error: (error: HttpErrorResponse) => this.showError(error)
      });
  }

  private showError(error: HttpErrorResponse): void {
    this.messages.add({ severity: 'error', summary: 'خطا', detail: userErrorMessage(error, 'انجام درخواست پروفایل ناموفق بود.') });
  }
}
