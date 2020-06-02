package validations

import (
	"testing"

	"github.com/kiali/kiali/models"
	"github.com/stretchr/testify/assert"
)

type TestAsserter interface {
	AssertNoValidations()
	AssertValidationsPresent(int)
	AssertValidationAt(int, models.SeverityLevel, string, string)
}

type ValidationTestAsserter struct {
	T           *testing.T
	Validations []*models.IstioCheck
	Valid       bool
}

func (tb ValidationTestAsserter) AssertNoValidations() {
	assert := assert.New(tb.T)

	assert.Empty(tb.Validations)
	assert.True(tb.Valid)
}

func (tb ValidationTestAsserter) AssertValidationsPresent(count int) {
	assert := assert.New(tb.T)

	assert.False(tb.Valid)
	assert.NotEmpty(tb.Validations)
	assert.Len(tb.Validations, count)
}

func (tb ValidationTestAsserter) AssertValidationAt(i int, severity models.SeverityLevel, path, message string) {
	assert := assert.New(tb.T)

	if len(tb.Validations) < i {
		tb.T.Error("Wrong memory access to validations array")
	}

	validation := tb.Validations[i]
	assert.NotNil(validation)
	assert.Equal(severity, validation.Severity)
	assert.Equal(path, validation.Path)
	assert.Equal(models.CheckMessage(message), validation.Message)
}
