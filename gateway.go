package gopay

import (
	"context"
	"fmt"
	"net/http"
)

type Driver interface {
	GetName() string
}

type RedirectPayer interface {
	Purchase(ctx context.Context, req *TransactionRequest) (*PaymentResponse, error)
	VerifyAndConfirm(ctx context.Context, r *http.Request, fetcher TransactionFetcher) (*VerificationResponse, error)
}
type PaymentResponse struct {
	// فیلدهای درخواستی شما
	Success        bool              `json:"success"`              // آیا درخواست موفق بود
	Message        string            `json:"message"`              // پیام خطا یا موفقیت
	PaymentURL     string            `json:"paymentURL,omitempty"` // آدرس درگاه (جایگزین PaymentURL)
	Authority      string            `json:"authority,omitempty"`
	RedirectMethod string            `json:"redirectMethod,omitempty"` // متد هدایت کاربر ("GET" or "POST")
	RedirectParams map[string]string `json:"redirectParams,omitempty"` // پارامترها (مخصوصاً برای POST)
}

type Refundable interface {
	Refund(ctx context.Context, req *RefundRequest) (*RefundResponse, error)
}

type TransactionRequest struct {
	Amount         int64
	CallbackURL    string
	Description    string
	IdempotencyKey string
}

type TransactionFetcher func(ctx context.Context, authority string) (*OriginalTransaction, error)

type OriginalTransaction struct {
	Amount int64
}

type VerificationStatus int

const (
	StatusFailed VerificationStatus = iota
	StatusSuccess
	StatusAlreadyVerified
	StatusAmountMismatch
	StatusCancelled
	StatusInvalid
)

type VerificationResponse struct {
	Status       VerificationStatus
	ReferenceID  string
	CardNumber   string
	Message      string
	OriginalData map[string]interface{}
}

type RefundRequest struct {
	TransactionRefID string
	Amount           int64
}

type RefundResponse struct {
	IsSuccess bool
}

type GatewayError struct {
	Code    int
	Message string
	Err     error
}

func (e *GatewayError) Error() string {
	return fmt.Sprintf("gateway error: code=%d, msg='%s', underlying_err=%v", e.Code, e.Message, e.Err)
}
