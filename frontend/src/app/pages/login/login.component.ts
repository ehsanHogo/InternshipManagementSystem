import { HttpErrorResponse } from '@angular/common/http';
import { Component, inject } from '@angular/core';
import { FormBuilder, ReactiveFormsModule, Validators } from '@angular/forms';
import { Router } from '@angular/router';
import { finalize } from 'rxjs';
import { MessageService } from 'primeng/api';
import { ButtonModule } from 'primeng/button';
import { CardModule } from 'primeng/card';
import { InputTextModule } from 'primeng/inputtext';
import { PasswordModule } from 'primeng/password';

import { AuthService } from '../../auth/auth.service';

@Component({
  selector: 'app-login',
  imports: [ReactiveFormsModule, ButtonModule, CardModule, InputTextModule, PasswordModule],
  templateUrl: './login.component.html',
  styleUrl: './login.component.scss'
})
export class LoginComponent {
  private readonly formBuilder = inject(FormBuilder);
  private readonly auth = inject(AuthService);
  private readonly router = inject(Router);
  private readonly messages = inject(MessageService);

  submitting = false;
  readonly loginForm = this.formBuilder.nonNullable.group({
    email: ['', [Validators.required, Validators.email]],
    password: ['', Validators.required]
  });

  submit(): void {
    if (this.loginForm.invalid) {
      this.loginForm.markAllAsTouched();
      return;
    }

    this.submitting = true;
    const { email, password } = this.loginForm.getRawValue();
    this.auth
      .login(email.trim(), password)
      .pipe(finalize(() => (this.submitting = false)))
      .subscribe({
        next: () => void this.router.navigateByUrl('/dashboard'),
        error: (error: HttpErrorResponse) => this.showLoginError(error)
      });
  }

  private showLoginError(error: HttpErrorResponse): void {
    let detail = 'خطایی غیرمنتظره رخ داد. لطفاً دوباره تلاش کنید.';
    if (error.status === 0) {
      detail = 'خطا در ارتباط با سرور';
    } else if (error.status === 401) {
      detail = 'ایمیل یا رمز عبور صحیح نیست';
    }

    this.messages.add({ severity: 'error', summary: 'ورود ناموفق', detail });
  }
}
