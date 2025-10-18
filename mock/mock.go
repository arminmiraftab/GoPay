package mock

import (
	"context"
	"gopay"
	"net/http"
)

type Driver struct {
	OnPurchase func(ctx context.Context, req *gopay.TransactionRequest) (*gopay.PaymentResponse, error)
	OnVerify   func(ctx context.Context, r *http.Request, fetcher gopay.TransactionFetcher) (*gopay.VerificationResponse, error)
}

var _ gopay.Driver = (*Driver)(nil)
var _ gopay.RedirectPayer = (*Driver)(nil)

func (m *Driver) GetName() string { return "mock" }

func (m *Driver) Purchase(ctx context.Context, req *gopay.TransactionRequest) (*gopay.PaymentResponse, error) {
	return m.OnPurchase(ctx, req)
}

func (m *Driver) VerifyAndConfirm(ctx context.Context, r *http.Request, fetcher gopay.TransactionFetcher) (*gopay.VerificationResponse, error) {
	return m.OnVerify(ctx, r, fetcher)
}
