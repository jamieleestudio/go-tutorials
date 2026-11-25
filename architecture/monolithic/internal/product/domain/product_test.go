package domain_test

import (
	"testing"

	productdomain "github.com/jamieleestudio/go-tutorials/architecture/monolithic/internal/product/domain"
	"github.com/jamieleestudio/go-tutorials/architecture/monolithic/internal/shared"
	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCreateProductSuccessfully(t *testing.T) {
	product, err := productdomain.Create("Laptop", decimal.RequireFromString("999.99"), 10)
	require.NoError(t, err)

	assert.NotEmpty(t, product.ID())
	assert.Equal(t, "Laptop", product.Name())
	assert.True(t, product.Price().Equal(decimal.RequireFromString("999.99")))
	assert.Equal(t, 10, product.Stock())
}

func TestCreateProductValidations(t *testing.T) {
	cases := []struct {
		name       string
		productName string
		price      string
		stock      int
		wantCode   string
	}{
		{"blank name", "", "10.00", 1, "INVALID_PRODUCT"},
		{"zero price", "Laptop", "0", 1, "INVALID_PRODUCT"},
		{"negative price", "Laptop", "-1", 1, "INVALID_PRODUCT"},
		{"negative stock", "Laptop", "10.00", -1, "INVALID_PRODUCT"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := productdomain.Create(tc.productName, decimal.RequireFromString(tc.price), tc.stock)
			require.Error(t, err)
			var domErr *shared.DomainError
			require.ErrorAs(t, err, &domErr)
			assert.Equal(t, tc.wantCode, domErr.Code)
		})
	}
}

func TestDeductStockSuccess(t *testing.T) {
	product, err := productdomain.Create("Laptop", decimal.RequireFromString("999.99"), 10)
	require.NoError(t, err)

	require.NoError(t, product.DeductStock(3))
	assert.Equal(t, 7, product.Stock())
}

func TestDeductStockInsufficient(t *testing.T) {
	product, err := productdomain.Create("Laptop", decimal.RequireFromString("999.99"), 5)
	require.NoError(t, err)

	err = product.DeductStock(20)
	require.Error(t, err)
	var domErr *shared.DomainError
	require.ErrorAs(t, err, &domErr)
	assert.Equal(t, "OUT_OF_STOCK", domErr.Code)
}

func TestRestock(t *testing.T) {
	product, err := productdomain.Create("Laptop", decimal.RequireFromString("999.99"), 5)
	require.NoError(t, err)

	require.NoError(t, product.Restock(10))
	assert.Equal(t, 15, product.Stock())
	assert.Error(t, product.Restock(0))
}

func TestChangePrice(t *testing.T) {
	product, err := productdomain.Create("Laptop", decimal.RequireFromString("999.99"), 5)
	require.NoError(t, err)

	require.NoError(t, product.ChangePrice(decimal.RequireFromString("899.99")))
	assert.True(t, product.Price().Equal(decimal.RequireFromString("899.99")))
	assert.Error(t, product.ChangePrice(decimal.Zero))
}
