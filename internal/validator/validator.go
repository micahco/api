package validator

import (
	"bytes"
	"fmt"
	"regexp"
	"strings"
	"unicode/utf8"
)

var (
	// https://html.spec.whatwg.org/#valid-e-mail-address
	EmailRX = regexp.MustCompile("^[a-zA-Z0-9.!#$%&'*+\\/=?^_`{|}~-]+@[a-zA-Z0-9](?:[a-zA-Z0-9-]{0,61}[a-zA-Z0-9])?(?:\\.[a-zA-Z0-9](?:[a-zA-Z0-9-]{0,61}[a-zA-Z0-9])?)*$")
	PhoneRX = regexp.MustCompile(`^[\+\(\s.\-\/\d\)]{5,30}$`)
)

type Validator struct {
	messages []string
}

func (v *Validator) IsValid() bool {
	return len(v.messages) == 0
}

func (v *Validator) Errors() string {
	b := new(bytes.Buffer)
	for _, message := range v.messages {
		fmt.Fprintln(b, message)
	}
	return b.String()
}

func (v *Validator) Validate(ok bool, message string) {
	if !ok {
		v.messages = append(v.messages, message)
	}
}

func NotBlank(value string) bool {
	return strings.TrimSpace(value) != ""
}

func MaxChars(value string, n int) bool {
	return utf8.RuneCountInString(value) <= n
}

func MinChars(value string, n int) bool {
	return utf8.RuneCountInString(value) >= n
}

func Matches(value string, rx *regexp.Regexp) bool {
	return rx.MatchString(value)
}

func PermittedInt(value int, permittedValues ...int) bool {
	for i := range permittedValues {
		if value == permittedValues[i] {
			return true
		}
	}
	return false
}
