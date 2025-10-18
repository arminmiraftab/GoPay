package main

import (
	"context"
	"fmt"
	"gopay"
	"gopay/drivers/parsian_v1"
	"log"
	"net/http"
	"time"
)

func main() {
	// تنظیمات درگاه پارسیان
	config := &gopay.Config{
		Drivers: map[string]gopay.DriverConfig{
			"parsian_v1": {
				"login_account": "8v5fyU1F7sMT8kR5YR3u", // LoginAccount ارائه‌شده
			},
		},
	}

	// ایجاد کلاینت
	client := gopay.NewClient(config)

	// ثبت درایور پارسیان
	client.Register("parsian_v1", parsian_v1.Initializer)

	// تنظیم سرور HTTP
	http.HandleFunc("/purchase", handlePurchase(client))
	http.HandleFunc("/callback", handleCallback(client))
	log.Println("Server running on :8080")
	log.Fatal(http.ListenAndServe(":8080", nil))
}

// هندلر برای شروع تراکنش
func handlePurchase(client *gopay.Client) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// دریافت درایور
		driver, err := client.GetDriver("parsian_v1")
		if err != nil {
			http.Error(w, fmt.Sprintf("Failed to get driver: %v", err), http.StatusInternalServerError)
			return
		}

		// ایجاد درخواست پرداخت
		req := &gopay.TransactionRequest{
			Amount:         10000,                            // مبلغ به ریال (مثال: 10,000 ریال)
			CallbackURL:    "http://localhost:8080/callback", // برای تست محلی
			Description:    "Test payment",
			IdempotencyKey: "order-" + fmt.Sprintf("%d", time.Now().UnixNano()), // شناسه یکتا
		}

		// شروع تراکنش
		resp, err := driver.(gopay.RedirectPayer).Purchase(r.Context(), req)
		if err != nil {
			http.Error(w, fmt.Sprintf("Purchase failed: %v", err), http.StatusInternalServerError)
			return
		}

		// هدایت کاربر به صفحه پرداخت
		http.Redirect(w, r, resp.PaymentURL, http.StatusSeeOther)
	}
}

// هندلر برای callback بعد از پرداخت
func handleCallback(client *gopay.Client) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// دریافت درایور
		driver, err := client.GetDriver("parsian_v1")
		if err != nil {
			http.Error(w, fmt.Sprintf("Failed to get driver: %v", err), http.StatusInternalServerError)
			return
		}

		// تابع fetcher برای دریافت اطلاعات تراکنش اصلی (نمونه ساده)
		fetcher := func(ctx context.Context, authority string) (*gopay.OriginalTransaction, error) {
			// اینجا باید از دیتابیس یا ذخیره‌سازی اطلاعات تراکنش را بگیرید
			return &gopay.OriginalTransaction{Amount: 10000}, nil
		}

		// تأیید تراکنش
		verifyResp, err := driver.(gopay.RedirectPayer).VerifyAndConfirm(r.Context(), r, fetcher)
		if err != nil {
			http.Error(w, fmt.Sprintf("Verification failed: %v", err), http.StatusInternalServerError)
			return
		}

		// بررسی وضعیت تأیید
		switch verifyResp.Status {
		case gopay.StatusSuccess:
			fmt.Fprintf(w, "Payment successful! Reference ID: %s, Card: %s", verifyResp.ReferenceID, verifyResp.CardNumber)
		case gopay.StatusFailed:
			fmt.Fprintf(w, "Payment failed!")
		case gopay.StatusCancelled:
			fmt.Fprintf(w, "Payment cancelled by user!")
		case gopay.StatusAmountMismatch:
			fmt.Fprintf(w, "Amount mismatch!")
		case gopay.StatusAlreadyVerified:
			fmt.Fprintf(w, "Transaction already verified!")
		default:
			fmt.Fprintf(w, "Invalid payment status!")
		}
	}
}
