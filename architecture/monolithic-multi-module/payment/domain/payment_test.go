package domain_test

import (
	"testing"

	paymentdomain "github.com/jamieleestudio/go-tutorials/architecture/monolithic-multi-module/payment/domain"
	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func samplePayment(t *testing.T) *paymentdomain.Payment {
	t.Helper()
	p, err := paymentdomain.Create("order-1", decimal.RequireFromString("1999.98"))
	require.NoError(t, err)
	return p
}

func TestCreatePayment(t *testing.T) {
	p := samplePayment(t)
	assert.NotEmpty(t, p.ID())
	assert.Equal(t, "order-1", p.OrderID())
	assert.Equal(t, paymentdomain.StatusPending, p.Status())
	assert.True(t, p.Amount().Equal(decimal.RequireFromString("1999.98")))
}

func TestCreatePaymentValidations(t *testing.T) {
	_, err := paymentdomain.Create("", decimal.RequireFromString("10"))
	assert.Error(t, err)
	_, err = paymentdomain.Create("order-1", decimal.Zero)
	assert.Error(t, err)
}

func TestPaymentSucceed(t *testing.T) {
	p := samplePayment(t)
	require.NoError(t, p.Succeed())
	assert.Equal(t, paymentdomain.StatusSuccess, p.Status())
	assert.Error(t, p.Succeed()) // PENDING → SUCCESS 只允许一次
}

func TestPaymentFail(t *testing.T) {
	p := samplePayment(t)
	require.NoError(t, p.Fail("card declined"))
	assert.Equal(t, paymentdomain.StatusFailed, p.Status())
	assert.Equal(t, "card declined", p.FailureReason())
	assert.Error(t, p.Refund()) // FAILED 状态不能退款
}

func TestPaymentRefund(t *testing.T) {
	p := samplePayment(t)
	require.NoError(t, p.Succeed())
	require.NoError(t, p.Refund())
	assert.Equal(t, paymentdomain.StatusRefunded, p.Status())
}
