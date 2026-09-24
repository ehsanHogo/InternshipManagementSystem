import { HttpErrorResponse } from '@angular/common/http';
import { Component, inject } from '@angular/core';
import { FormBuilder, ReactiveFormsModule, Validators } from '@angular/forms';
import { Router, RouterLink } from '@angular/router';
import { finalize } from 'rxjs';
import { MessageService } from 'primeng/api';
import { ButtonModule } from 'primeng/button';
import { CardModule } from 'primeng/card';
import { InputTextModule } from 'primeng/inputtext';
import { PasswordModule } from 'primeng/password';
import { TextareaModule } from 'primeng/textarea';

import { CompanyAccountService } from '../../company/company-account.service';
import { CompanyRegistrationPayload } from '../../company/company-account.models';
import { userErrorMessage } from '../../shared/http-error-message';

const registrationErrorMessages: Record<string, string> = {
  INVALID_REGISTRATION: 'اطلاعات واردشده کامل یا معتبر نیست.',
  EMAIL_ALREADY_EXISTS: 'این ایمیل قبلاً در سامانه ثبت شده است.',
  NATIONAL_ID_ALREADY_EXISTS: 'شرکتی با این شناسه ملی قبلاً ثبت شده است.',
  ECONOMIC_CODE_ALREADY_EXISTS: 'شرکتی با این کد اقتصادی قبلاً ثبت شده است.',
  COMPANY_NAME_ALREADY_EXISTS: 'شرکتی با این نام قبلاً ثبت شده است.'
};

@Component({
  selector: 'app-company-register',
  imports: [
    ReactiveFormsModule,
    RouterLink,
    ButtonModule,
    CardModule,
    InputTextModule,
    PasswordModule,
    TextareaModule
  ],
  templateUrl: './company-register.component.html',
  styleUrl: './company-register.component.scss'
})
export class CompanyRegisterComponent {
  private readonly formBuilder = inject(FormBuilder);
  private readonly companyAccount = inject(CompanyAccountService);
  private readonly messages = inject(MessageService);
  private readonly router = inject(Router);

  submitting = false;
  readonly registrationForm = this.formBuilder.nonNullable.group({
    supervisor: this.formBuilder.nonNullable.group({
      fullName: ['', Validators.required],
      email: ['', [Validators.required, Validators.email]],
      password: ['', Validators.required],
      phone: ['', Validators.required],
      jobTitle: ['', Validators.required]
    }),
    company: this.formBuilder.nonNullable.group({
      name: ['', Validators.required],
      nationalId: ['', Validators.required],
      economicCode: ['', Validators.required],
      website: [''],
      phone: ['', Validators.required],
      email: ['', [Validators.required, Validators.email]],
      address: ['', Validators.required]
    })
  });

  submit(): void {
    if (this.submitting) return;
    if (this.registrationForm.invalid) {
      this.registrationForm.markAllAsTouched();
      this.messages.add({
        severity: 'warn',
        summary: 'اطلاعات ناقص',
        detail: 'لطفاً همه فیلدهای الزامی را به‌درستی تکمیل کنید.'
      });
      return;
    }

    this.submitting = true;
    const value = this.registrationForm.getRawValue();
    const payload: CompanyRegistrationPayload = {
      supervisor: {
        fullName: value.supervisor.fullName.trim(),
        email: value.supervisor.email.trim(),
        password: value.supervisor.password,
        phone: value.supervisor.phone.trim(),
        jobTitle: value.supervisor.jobTitle.trim()
      },
      company: {
        name: value.company.name.trim(),
        nationalId: value.company.nationalId.trim(),
        economicCode: value.company.economicCode.trim(),
        website: value.company.website.trim(),
        phone: value.company.phone.trim(),
        email: value.company.email.trim(),
        address: value.company.address.trim()
      }
    };

    this.companyAccount
      .register(payload)
      .pipe(finalize(() => (this.submitting = false)))
      .subscribe({
        next: () => {
          this.messages.add({
            severity: 'success',
            summary: 'ثبت‌نام موفق',
            detail: 'ثبت‌نام شرکت با موفقیت انجام شد. اکنون می‌توانید وارد سامانه شوید.'
          });
          void this.router.navigateByUrl('/login');
        },
        error: (error: HttpErrorResponse) => {
          const code = typeof error.error?.code === 'string' ? error.error.code : '';
          this.messages.add({
            severity: 'error',
            summary: 'ثبت‌نام ناموفق',
            detail:
              registrationErrorMessages[code] ??
              userErrorMessage(error, 'ثبت‌نام شرکت انجام نشد. لطفاً دوباره تلاش کنید.')
          });
        }
      });
  }
}
