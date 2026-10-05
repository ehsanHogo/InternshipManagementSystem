package model

type CompanyRegistrationStatus string

const (
	CompanyRegistrationStatusPending  CompanyRegistrationStatus = "PENDING"
	CompanyRegistrationStatusApproved CompanyRegistrationStatus = "APPROVED"
	CompanyRegistrationStatusRejected CompanyRegistrationStatus = "REJECTED"
)

func (status CompanyRegistrationStatus) Valid() bool {
	return status == CompanyRegistrationStatusPending || status == CompanyRegistrationStatusApproved || status == CompanyRegistrationStatusRejected
}
