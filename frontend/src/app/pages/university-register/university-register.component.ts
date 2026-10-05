import { Component, inject, signal } from '@angular/core';
import { FormBuilder, ReactiveFormsModule, Validators } from '@angular/forms';
import { RouterLink } from '@angular/router';
import { ButtonModule } from 'primeng/button';
import { InputTextModule } from 'primeng/inputtext';
import { finalize } from 'rxjs';
import { UserAccessService } from '../../user-access/user-access.service';
import { userErrorMessage } from '../../shared/http-error-message';

@Component({
 selector: 'app-university-register',
 imports: [ReactiveFormsModule, RouterLink, ButtonModule, InputTextModule],
 templateUrl: './university-register.component.html',
 styleUrl: './university-register.component.scss'
})
export class UniversityRegisterComponent {
 private readonly access = inject(UserAccessService);
 private readonly fb = inject(FormBuilder);
 readonly busy = signal(false);
 readonly registered = signal(false);
 readonly error = signal('');
 readonly form = this.fb.nonNullable.group({
  fullName: ['', [Validators.required, Validators.maxLength(200)]],
  email: ['', [Validators.required, Validators.email, Validators.maxLength(320)]],
  phone: ['', Validators.maxLength(50)],
  password: ['', Validators.required]
 });
 submit(): void {
  if (this.busy()) return;
  if (this.form.invalid || !this.form.controls.fullName.value.trim() || !this.form.controls.password.value.trim() || new TextEncoder().encode(this.form.controls.password.value).length > 72) {
   this.form.markAllAsTouched(); this.error.set('نام، ایمیل معتبر و رمز عبور غیرخالی تا ۷۲ بایت الزامی است.'); return;
  }
  this.busy.set(true); this.error.set('');
  this.access.register(this.form.getRawValue()).pipe(finalize(() => this.busy.set(false))).subscribe({
   next: () => { this.form.reset(); this.registered.set(true); },
   error: error => this.error.set(userErrorMessage(error, 'ثبت‌نام ناموفق بود.'))
  });
 }
}
