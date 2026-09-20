import {
  ApplicationConfig,
  inject,
  provideAppInitializer,
  provideZoneChangeDetection
} from '@angular/core';
import { provideHttpClient, withInterceptors } from '@angular/common/http';
import { provideAnimationsAsync } from '@angular/platform-browser/animations/async';
import { provideRouter } from '@angular/router';
import { providePrimeNG } from 'primeng/config';
import { MessageService } from 'primeng/api';
import Aura from '@primeng/themes/aura';

import { routes } from './app.routes';
import { authInterceptor } from './auth/auth.interceptor';
import { AuthService } from './auth/auth.service';

export const appConfig: ApplicationConfig = {
  providers: [
    provideZoneChangeDetection({ eventCoalescing: true }),
    provideRouter(routes),
    provideHttpClient(withInterceptors([authInterceptor])),
    provideAppInitializer(() => inject(AuthService).initialize()),
    provideAnimationsAsync(),
    MessageService,
    providePrimeNG({
      ripple: true,
      overlayAppendTo: 'body',
      zIndex: {
        menu: 1050,
        overlay: 1100,
        modal: 1200,
        tooltip: 1450
      },
      translation: {
        accept: 'تأیید',
        reject: 'انصراف',
        choose: 'انتخاب',
        upload: 'بارگذاری',
        cancel: 'انصراف',
        clear: 'پاک کردن',
        apply: 'اعمال',
        today: 'امروز',
        emptyMessage: 'موردی یافت نشد.',
        emptyFilterMessage: 'نتیجه‌ای یافت نشد.',
        noFileChosenMessage: 'فایلی انتخاب نشده است.',
        fileChosenMessage: 'فایل انتخاب شد.',
        searchMessage: '{0} نتیجه یافت شد.',
        emptySearchMessage: 'نتیجه‌ای یافت نشد.',
        passwordPrompt: 'رمز عبور را وارد کنید.',
        weak: 'ضعیف',
        medium: 'متوسط',
        strong: 'قوی',
        aria: {
          close: 'بستن',
          previous: 'قبلی',
          next: 'بعدی',
          selectAll: 'انتخاب همه',
          unselectAll: 'لغو انتخاب همه'
        }
      },
      theme: {
        preset: Aura,
        options: {
          darkModeSelector: false
        }
      }
    })
  ]
};
