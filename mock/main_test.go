package mock

//
//import (
//	"context"
//	"fmt"
//	"gopay"
//	"net/http/httptest"
//	"testing"
//)
//
//// این یک تابع نمونه از منطق برنامه شماست که می‌خواهیم آن را تست کنیم
//func startPaymentProcess(payer gopay.RedirectPayer) (string, error) {
//	req := &gopay.TransactionRequest{
//		Amount:      50000,
//		CallbackURL: "http://localhost/callback",
//		Description: "تست خرید محصول",
//	}
//
//	resp, err := payer.Purchase(context.Background(), req)
//	if err != nil {
//		return "", fmt.Errorf("خطا در ایجاد پرداخت: %w", err)
//	}
//	return resp.PaymentURL, nil
//}
//
//// این تابع تست اصلی ماست
//func TestSuccessfulPayment(t *testing.T) {
//	// ۱. یک نمونه از درایور Mock می‌سازیم
//	mockDriver := &mock.Driver{}
//
//	// ۲. رفتار آن را تعریف می‌کنیم: به ما بگو وقتی Purchase صدا زده شد، چه چیزی برگردان
//	mockDriver.OnPurchase = func(ctx context.Context, req *gopay.TransactionRequest) (*gopay.PaymentResponse, error) {
//		// ما انتظار داریم که مبلغ ۵۰۰۰۰ ریال باشد
//		if req.Amount != 50000 {
//			t.Errorf("مبلغ ارسالی اشتباه است، انتظار %d را داشتیم ولی %d دریافت شد", 50000, req.Amount)
//		}
//
//		// یک پاسخ موفقیت‌آمیز جعلی برمی‌گردانیم
//		return &gopay.PaymentResponse{
//			Authority:  "MOCK-AUTH-12345",
//			PaymentURL: "http://mock-payment-url.com/MOCK-AUTH-12345",
//		}, nil
//	}
//
//	// ۳. منطق برنامه خود را با درایور Mock فراخوانی می‌کنیم
//	paymentURL, err := startPaymentProcess(mockDriver)
//	if err != nil {
//		t.Fatalf("انتظار خطا نداشتیم ولی این خطا دریافت شد: %v", err)
//	}
//
//	// ۴. نتیجه را بررسی می‌کنیم
//	expectedURL := "http://mock-payment-url.com/MOCK-AUTH-12345"
//	if paymentURL != expectedURL {
//		t.Errorf("URL پرداخت اشتباه است، انتظار '%s' را داشتیم ولی '%s' دریافت شد", expectedURL, paymentURL)
//	}
//
//	fmt.Println("✅ تست پرداخت موفق با موفقیت انجام شد.")
//}
//
//// شما می‌توانید تست‌های دیگری برای سناریوهای خطا بنویسید
//func TestFailedPayment(t *testing.T) {
//	mockDriver := &mock.Driver{}
//
//	// این بار، یک خطای جعلی برمی‌گردانیم
//	mockDriver.OnPurchase = func(ctx context.Context, req *gopay.TransactionRequest) (*gopay.PaymentResponse, error) {
//		return nil, fmt.Errorf("اتصال به درگاه برقرار نشد")
//	}
//
//	_, err := startPaymentProcess(mockDriver)
//	if err == nil {
//		t.Fatal("انتظار خطا داشتیم ولی خطایی دریافت نشد")
//	}
//
//	fmt.Println("✅ تست پرداخت ناموفق با موفقیت انجام شد.")
//}
