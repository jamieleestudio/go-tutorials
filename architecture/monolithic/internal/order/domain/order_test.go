package domain_test

import (
	"testing"

	"github.com/jamieleestudio/go-tutorials/architecture/monolithic/internal/order/domain"
	"github.com/jamieleestudio/go-tutorials/architecture/monolithic/internal/shared"
	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// 对应 Java 版 order.domain.OrderTest 的 8 个用例。

func mustItem(t *testing.T, productID, productName string, quantity int, price string) domain.OrderItem {
	t.Helper()
	item, err := domain.NewOrderItem(productID, productName, quantity, decimal.RequireFromString(price))
	require.NoError(t, err)
	return item
}

func sampleOrder(t *testing.T) *domain.Order {
	t.Helper()
	order, err := domain.Create("c1", []domain.OrderItem{mustItem(t, "p1", "Laptop", 1, "100")})
	require.NoError(t, err)
	return order
}

func TestCreateOrderSuccessfully(t *testing.T) {
	order, err := domain.Create("c1", []domain.OrderItem{mustItem(t, "p1", "Laptop", 2, "999.99")})
	require.NoError(t, err)

	assert.NotEmpty(t, order.ID())
	assert.Equal(t, "c1", order.CustomerID())
	assert.Equal(t, domain.StatusCreated, order.Status())
	assert.Len(t, order.Items(), 1)
	assert.True(t, order.TotalAmount().Equal(decimal.RequireFromString("1999.98")))
	assert.Len(t, order.DomainEvents(), 1)
}

func TestCreateOrderWithEmptyItemsThrows(t *testing.T) {
	_, err := domain.Create("c1", nil)
	require.Error(t, err)
	var domErr *shared.DomainError
	require.ErrorAs(t, err, &domErr)
	assert.Equal(t, shared.KindBusinessRule, domErr.Kind)
	assert.Equal(t, "INVALID_ORDER", domErr.Code)
}

func TestCreateOrderWithBlankCustomerThrows(t *testing.T) {
	_, err := domain.Create("", []domain.OrderItem{mustItem(t, "p1", "Laptop", 1, "100")})
	require.Error(t, err)
}

func TestMarkAsPaidTransitionsToPaid(t *testing.T) {
	order := sampleOrder(t)
	require.NoError(t, order.MarkAsPaid("pay-1"))

	assert.Equal(t, domain.StatusPaid, order.Status())
	assert.Equal(t, "pay-1", order.PaymentID())
	assert.Len(t, order.DomainEvents(), 2) // Created + Paid
}

func TestCannotPayAlreadyPaidOrder(t *testing.T) {
	order := sampleOrder(t)
	require.NoError(t, order.MarkAsPaid("pay-1"))
	require.Error(t, order.MarkAsPaid("pay-2"))
}

func TestCancelCreatedOrder(t *testing.T) {
	order := sampleOrder(t)
	require.NoError(t, order.Cancel("changed mind"))
	assert.Equal(t, domain.StatusCancelled, order.Status())
}

func TestConfirmPaidOrder(t *testing.T) {
	order := sampleOrder(t)
	require.NoError(t, order.MarkAsPaid("pay-1"))
	require.NoError(t, order.Confirm())
	assert.Equal(t, domain.StatusConfirmed, order.Status())
}

func TestCannotCancelConfirmedOrder(t *testing.T) {
	order := sampleOrder(t)
	require.NoError(t, order.MarkAsPaid("pay-1"))
	require.NoError(t, order.Confirm())
	assert.Error(t, order.Cancel("reason"))
}
