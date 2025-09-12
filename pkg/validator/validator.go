package validator

import "regexp"

type Validator struct {
	Errors map[string]string
}

func New() *Validator {
	return &Validator{Errors: make(map[string]string)}
}

func (v *Validator) Valid() bool {
	return len(v.Errors) == 0
}

func (v *Validator) AddError(key, message string) {
	if _, exists := v.Errors[key]; !exists {
		v.Errors[key] = message
	}
}

func (v *Validator) Check(ok bool, key, message string) {
	if !ok {
		v.AddError(key, message)
	}
}

// IsEmailValid checks if the provided email string is a valid email address format.
func (v *Validator) IsEmailValid(email, key, message string) {
	// Simple regex for email validation (RFC 5322 official standard is more complex)
	// This covers most common valid emails.
	var emailRegex = `^[a-zA-Z0-9._%%+-]+@[a-zA-Z0-9.-]+\.[a-zA-Z]{2,}$`
	matched := false
	if len(email) > 0 {
		matched = regexp.MustCompile(emailRegex).MatchString(email)
	}
	if !matched {
		v.AddError(key, message)
	}
}