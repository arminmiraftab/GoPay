package main

import (
	"context"
	"fmt"
	"gopay" // پکیج اصلی شما
	"log"
	"net/http"

	// (مسیر ایمپورت را بر اساس نام ماژول go.mod خودتان تنظیم کنید)
	behpardakht "gopay/drivers/behpardakht_v1"
)

// client را به یک متغیر سراسری تبدیل می‌کنیم تا در تمام هندلرها قابل دسترس باشد
var gopayClient *gopay.Client

// ۲. شبیه‌سازی خواندن تنظیمات
func getConfig() *gopay.Config {
	return &gopay.Config{
		Drivers: map[string]gopay.DriverConfig{
			"mellat_main": {
				"terminal_id": "123456", // !! مقادیر واقعی خود را جایگزین کنید !!
				"username":    "your_mellat_user",
				"password":    "your_mellat_pass",
			},
		},
	}
}

func main() {
	fmt.Println("Starting GoPay client...")
	config := getConfig()

	// ۴. ساخت کلاینت اصلی
	gopayClient = gopay.NewClient(config)

	// ۵. ثبت (Register) کردن درایور
	if err := gopayClient.Register("mellat_main", behpardakht.Initializer); err != nil {
		log.Fatalf("FATAL: Failed to register driver 'mellat_main': %v", err)
	} else {
		fmt.Println("Driver 'mellat_main' registered successfully.")
	}

	// ۶. تعریف مسیرهای (Routes) وب سرور
	http.HandleFunc("/pay", handlePay)
	http.HandleFunc("/payment/callback", handleCallback)

	// ۷. اجرای وب سرور
	log.Println("Starting web server on http://localhost:8080")
	if err := http.ListenAndServe(":8080", nil); err != nil {
		log.Fatalf("FATAL: Could not start server: %v", err)
	}
}

// handlePay وظیفه ایجاد تراکنش و هدایت کاربر به بانک را دارد
func handlePay(w http.ResponseWriter, r *http.Request) {
	log.Println("Received request for /pay")

	// ۱. دریافت درایور
	driver, err := gopayClient.GetDriver("mellat_main")
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// ۲. تبدیل درایور به نوع قابل پرداخت
	paymentDriver, ok := driver.(gopay.RedirectPayer)
	if !ok {
		http.Error(w, "Driver does not support RedirectPayer interface", http.StatusInternalServerError)
		return
	}

	// ۳. ساخت درخواست پرداخت
	// !! در یک برنامه واقعی، این مقادیر از دیتابیس یا سبد خرید می‌آیند !!
	orderID := "123456789" // این باید یک شناسه یکتای سفارش از دیتابیس شما باشد
	amount := int64(10000) // 1000 تومان (مبلغ به ریال است)

	req := &gopay.TransactionRequest{
		Amount:         amount,
		CallbackURL:    "http://localhost:8080/payment/callback", // آدرس کامل همین سرور
		Description:    "تست پرداخت با GoPay",
		IdempotencyKey: orderID, // همان OrderId
	}

	// ۴. صدا زدن Purchase
	resp, err := paymentDriver.Purchase(r.Context(), req)
	if err != nil {
		// اگر خطا از سمت درگاه باشد
		if gwErr, ok := err.(*gopay.GatewayError); ok {
			log.Printf("Gateway Error: %s (Code: %d)", gwErr.Message, gwErr.Code)
			http.Error(w, gwErr.Message, http.StatusInternalServerError)
		} else {
			log.Printf("Unknown Error: %v", err)
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}
		return
	}

	// ۵. هدایت کاربر به درگاه (مخصوص به پرداخت ملت)
	// به پرداخت ملت نیاز به ارسال `RefId` با متد POST دارد.
	// ما یک فرم HTML می‌سازیم که خودکار POST می‌شود.
	log.Printf("Redirecting user to bank... RefID: %s", resp.Authority)
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	fmt.Fprintf(w, `
	<html>
		<body onload="document.forms[0].submit()">
			<form action="%s" method="POST">
				<input type="hidden" name="RefId" value="%s" />
				<p>در حال انتقال به درگاه بانک... لطفا صبر کنید.</p>
				<button type="submit">انتقال به درگاه</button>
			</form>
		</body>
	</html>`, resp.PaymentURL, resp.Authority)
}

// handleCallback وظیفه دریافت پاسخ از بانک و تایید (Verify) تراکنش را دارد
func handleCallback(w http.ResponseWriter, r *http.Request) {
	log.Println("Received request for /payment/callback")

	// ۱. دریافت درایور
	driver, err := gopayClient.GetDriver("mellat_main")
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	paymentDriver, ok := driver.(gopay.RedirectPayer)
	if !ok {
		http.Error(w, "Driver does not support RedirectPayer interface", http.StatusInternalServerError)
		return
	}

	// ۲. ساخت TransactionFetcher (مهم‌ترین بخش طراحی شما)
	// این تابع به کتابخانه شما می‌گوید که چگونه تراکنش اصلی را از دیتابیس *شما* بخواند
	fetcher := func(ctx context.Context, authority string) (*gopay.OriginalTransaction, error) {
		// `authority` در به پرداخت، همان `SaleOrderId` (یعنی "123456789") است
		log.Printf("Fetcher: Trying to find transaction with OrderID: %s", authority)

		// !! در یک برنامه واقعی، اینجا باید از دیتابیس بخوانید !!
		// SELECT Amount FROM orders WHERE id = ?
		if authority == "123456789" {
			return &gopay.OriginalTransaction{
				Amount: int64(10000), // مبلغ اصلی (به ریال)
			}, nil
		}
		return nil, fmt.Errorf("transaction (OrderID: %s) not found in our database", authority)
	}

	// ۳. صدا زدن VerifyAndConfirm
	// 'r' همان http.Request است که از بانک آمده
	verifyResp, err := paymentDriver.VerifyAndConfirm(r.Context(), r, fetcher)

	// ۴. بررسی نتیجه نهایی
	if err != nil {
		if gwErr, ok := err.(*gopay.GatewayError); ok {
			log.Printf("Gateway Verify Error: %s (Code: %d)", gwErr.Message, gwErr.Code)
			fmt.Fprintf(w, "پرداخت ناموفق بود. خطا: %s", gwErr.Message)
		} else {
			log.Printf("Unknown Verify Error: %v", err)
			fmt.Fprintf(w, "خطای سیستمی در تایید پرداخت: %v", err)
		}
		return
	}

	if verifyResp.Status == gopay.StatusSuccess {
		// **مهم**: اینجا جایی است که باید دیتابیس خود را آپدیت کنید
		// UPDATE orders SET status = 'paid', ref_id = ? WHERE id = ?
		log.Printf("Payment Success! RefID: %s", verifyResp.ReferenceID)
		fmt.Fprintf(w, "پرداخت موفقیت‌آمیز بود! شماره پیگیری: %s", verifyResp.ReferenceID)
	} else {
		log.Printf("Payment Failed. Status: %v", verifyResp.Status)
		fmt.Fprintf(w, "پرداخت ناموفق بود. وضعیت: %v", verifyResp.Status)
	}
}
