package main

import (
	"context"
	"fmt"
	"net/http"
	"net/url"

	"github.com/arminmiraftab/GoPay"
	"github.com/arminmiraftab/GoPay/drivers/parsian_v1"
)

func main() {
	ctx := context.Background()

	// تنظیمات درگاه پارسیان
	config := gopay.DriverConfig{
		"login_account": "",
	}

	// ساخت درایور پارسیان (خروجی: gopay.RedirectPayer)
	driver, err := parsian_v1.New(config)
	if err != nil {
		panic(err)
	}
	// مرحله ۱: خرید (Purchase)
	req := &gopay.TransactionRequest{
		Amount:         10000,
		CallbackURL:    "",
		Description:    "تست درگاه پارسیان",
		IdempotencyKey: "1234567",
	}

	resp, err := driver.Purchase(ctx, req)
	if err != nil {
		fmt.Println("❌ خطا در Purchase:", err)
		return
	}

	fmt.Printf("✅ توکن پرداخت: %s\n🔗 لینک پرداخت: %s\n", resp.Authority, resp.PaymentURL)

	// مرحله ۲: شبیه‌سازی VerifyAndConfirm بعد از بازگشت از بانک
	fakeForm := url.Values{}
	fakeForm.Set("Token", resp.Authority)
	fakeForm.Set("status", "0")

	httpReq := &http.Request{Form: fakeForm}

	verifyResp, err := driver.VerifyAndConfirm(ctx, httpReq, func(ctx context.Context, token string) (*gopay.OriginalTransaction, error) {

		return &gopay.OriginalTransaction{Amount: req.Amount}, nil
	})
	if err != nil {
		fmt.Println("❌ خطا در VerifyAndConfirm:", err)
		return
	}

	fmt.Printf("✅ تأیید پرداخت موفق\n📜 ReferenceID: %s\n💳 Card: %s\n", verifyResp.ReferenceID, verifyResp.CardNumber)
}
