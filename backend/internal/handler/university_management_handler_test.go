package handler

import (
	"errors"
	"testing"

	"internship-management-system/backend/internal/model"
	"internship-management-system/backend/internal/service"
)

func TestResolveHeaderIndexesNormalizesPersianHeaders(t *testing.T) {
	expected := []string{importHeaderFullName, importHeaderEmail, importHeaderStudentNumber, importHeaderMajor}
	row := []string{"", "  نام و نام خانوادگي  ", "ايميل", "شماره دانشجويي", "رشته", ""}

	indexes, missing, valid := resolveHeaderIndexes(row, expected)
	if !valid || missing != "" {
		t.Fatalf("resolveHeaderIndexes() = valid %v, missing %q", valid, missing)
	}
	want := map[string]int{
		importHeaderFullName: 1, importHeaderEmail: 2,
		importHeaderStudentNumber: 3, importHeaderMajor: 4,
	}
	for header, index := range want {
		if indexes[header] != index {
			t.Errorf("index for %q = %d, want %d", header, indexes[header], index)
		}
	}
}

func TestResolveHeaderIndexesRejectsEnglishAndReportsMissingPersianHeader(t *testing.T) {
	expected := []string{importHeaderFullName, importHeaderEmail}
	_, missing, valid := resolveHeaderIndexes([]string{"full_name", "email"}, expected)
	if !valid {
		t.Fatal("English headers should be treated as missing, not structurally invalid")
	}
	if missing != importHeaderFullName {
		t.Fatalf("missing header = %q, want %q", missing, importHeaderFullName)
	}
}

func TestResolveHeaderIndexesRejectsDuplicateRequiredHeader(t *testing.T) {
	expected := []string{importHeaderFullName, importHeaderEmail}
	_, _, valid := resolveHeaderIndexes([]string{importHeaderFullName, importHeaderEmail, importHeaderEmail}, expected)
	if valid {
		t.Fatal("duplicate required header should make the structure invalid")
	}
}

func TestImportRowValidationMessages(t *testing.T) {
	tests := []struct {
		name  string
		role  model.Role
		input service.ManagedUserInput
		want  string
	}{
		{name: "full name", role: model.RoleProfessor, input: service.ManagedUserInput{Email: "user@example.com"}, want: "نام و نام خانوادگی وارد نشده است."},
		{name: "email", role: model.RoleProfessor, input: service.ManagedUserInput{FullName: "نام"}, want: "ایمیل وارد نشده است."},
		{name: "student number", role: model.RoleStudent, input: service.ManagedUserInput{FullName: "نام", Email: "user@example.com", Major: "رشته"}, want: "شماره دانشجویی وارد نشده است."},
		{name: "major", role: model.RoleStudent, input: service.ManagedUserInput{FullName: "نام", Email: "user@example.com", StudentNumber: "۴۰۱"}, want: "رشته تحصیلی وارد نشده است."},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := importRowValidationMessage(test.role, test.input); got != test.want {
				t.Fatalf("message = %q, want %q", got, test.want)
			}
		})
	}
}

func TestPublicImportErrorDoesNotExposeInternalErrors(t *testing.T) {
	if got := publicImportError(service.ErrEmailAlreadyExists); got != "کاربری با این ایمیل قبلاً ثبت شده است." {
		t.Fatalf("duplicate email message = %q", got)
	}
	if got := publicImportError(service.ErrStudentNumberExists); got != "دانشجویی با این شماره دانشجویی قبلاً ثبت شده است." {
		t.Fatalf("duplicate student number message = %q", got)
	}
	if got := publicImportError(errors.New("database connection details")); got != "اطلاعات این ردیف معتبر نیست." {
		t.Fatalf("internal error message = %q", got)
	}
}
