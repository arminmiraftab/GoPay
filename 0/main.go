package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"

	"gopay"                          // مسیر اصلی پکیج شما
	fanava "gopay/drivers/fanava_v1" // مسیری که فایل fanava.go در آن قرار دارد
)

// یک دیتابیس شبیه‌سازی شده برای ذخیره موقت مبلغ تراکنش
// در پروژه واقعی، شما باید Authority (توکن) را همراه با مبلغ و شماره سفارش در Redis یا DB ذخیره کنید.
var paymentDB = make(map[string]int64)

// کلاینت gopay که در کل برنامه استفاده خواهد شد
var payClient *gopay.Client

func main() {
	// ۱. تعریف تنظیمات
	cfg := &gopay.Config{
		Drivers: map[string]gopay.DriverConfig{
			"fanava": {
				"userID":   "211625298", // اطلاعات واقعی خود را جایگزین کنید
				"password": "211625298", // اطلاعات واقعی خود را جایگزین کنید
			},
		},
	}

	// ۲. ایجاد کلاینت
	payClient = gopay.NewClient(cfg)

	// ۳. ثبت درایور fanava
	// ما به کلاینت می‌گوییم که هر وقت کسی "fanava" را خواست،
	// از تابع سازنده NewFanava که در پکیج fanava_v1 است، استفاده کند.
	err := payClient.Register("fanava", fanava.NewFanava)
	if err != nil {
		log.Fatalf("خطا در ثبت درایور فن‌آوا: %v", err)
	}

	log.Println("درایور فن‌آوا با موفقیت ثبت شد.")

	// ۴. تعریف روت‌های HTTP
	http.HandleFunc("/pay", handlePurchase)
	http.HandleFunc("/callback", handleCallback)

	// ۵. اجرای سرور
	log.Println("سرور در حال اجرا روی پورت 8080...")
	log.Fatal(http.ListenAndServe(":8080", nil))
}

// handlePurchase وظیفه شروع فرآیند پرداخت را دارد
func handlePurchase(w http.ResponseWriter, r *http.Request) {
	// دریافت درایور "fanava" از کلاینت
	driver, err := payClient.GetDriver("fanava")
	if err != nil {
		http.Error(w, fmt.Sprintf("خطا در دریافت درایور: %v", err), 500)
		return
	}

	// ما انتظار داریم این درایور از نوع RedirectPayer باشد
	payer, ok := driver.(gopay.RedirectPayer)
	if !ok {
		http.Error(w, "درایور از نوع RedirectPayer نیست", 500)
		return
	}

	// مبلغ را برای سادگی ۱۰۰۰ تومان (۱۰,۰۰۰ ریال) در نظر می‌گیریم
	amount := int64(10000)

	// ساخت درخواست پرداخت
	txReq := &gopay.TransactionRequest{
		Amount:         amount,
		CallbackURL:    "http://localhost:8080/callback", // آدرسی که کاربر پس از پرداخت برمی‌گردد
		IdempotencyKey: fmt.Sprintf("order-%d", 12345),   // شماره فاکتور یا شناسه یکتای شما
		Description:    "خرید تستی",
	}

	// ۳. تماس با متد Purchase
	resp, err := payer.Purchase(r.Context(), txReq)
	if err != nil {
		http.Error(w, fmt.Sprintf("خطا در ایجاد تراکنش: %v", err), 500)
		return
	}

	// ۴. ذخیره اطلاعات تراکنش قبل از هدایت کاربر
	// این مرحله حیاتی است! ما باید بدانیم که این Authority (توکن) برای چه مبلغی بوده.
	paymentDB[resp.Authority] = amount
	log.Printf("تراکنش با توکن %s برای مبلغ %d ایجاد شد. در حال هدایت کاربر...", resp.Authority, amount)

	// ۵. ارسال پاسخ به فرانت‌اند
	// چون فن‌آوا نیاز به ریدایرکت POST دارد، بهترین راه ارسال JSON به فرانت‌اند است
	// تا فرانت‌اند یک فرم داینامیک بسازد و کاربر را ریدایرکت کند.
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}

// handleCallback وظیفه تایید نهایی پرداخت پس از بازگشت کاربر را دارد
func handleCallback(w http.ResponseWriter, r *http.Request) {
	log.Println("کاربر به Callback بازگشت...")

	// دریافت درایور "fanava"
	driver, err := payClient.GetDriver("fanava")
	if err != nil {
		http.Error(w, fmt.Sprintf("خطا در دریافت درایور: %v", err), 500)
		return
	}

	verifier, ok := driver.(gopay.RedirectPayer)
	if !ok {
		http.Error(w, "درایور از نوع RedirectPayer نیست", 500)
		return
	}

	// این تابع (Closure) حیاتی است.
	// پکیج gopay از این تابع استفاده می‌کند تا اطلاعات اصلی تراکنش را از دیتابیس شما بخواند.
	fetcher := func(ctx context.Context, authority string) (*gopay.OriginalTransaction, error) {
		log.Printf("فراخوانی TransactionFetcher برای توکن: %s", authority)

		// خواندن مبلغ از دیتابیس شبیه‌سازی شده ما
		amount, ok := paymentDB[authority]
		if !ok {
			// اگر توکن در دیتابیس ما نباشد، تراکنش نامعتبر است
			return nil, fmt.Errorf("تراکنش یافت نشد")
		}

		// برگرداندن اطلاعات تراکنش اصلی
		return &gopay.OriginalTransaction{
			Amount: amount,
		}, nil
	}

	// ۲. تایید تراکنش
	// ما r (که حاوی اطلاعات POST شده از درگاه است) و fetcher (تابع ما) را به پکیج می‌دهیم
	verifyResp, err := verifier.VerifyAndConfirm(r.Context(), r, fetcher)
	if err != nil {
		http.Error(w, fmt.Sprintf("خطا در تایید تراکنش: %v", err), 500)
		return
	}

	// ۳. بررسی وضعیت نهایی
	if verifyResp.Status == gopay.StatusSuccess {
		// پرداخت موفق بوده است
		// در اینجا باید دیتابیس خود را آپدیت کنید (مثلا سفارش را "پرداخت شده" ثبت کنید)
		log.Printf("پرداخت موفق! شماره پیگیری: %s", verifyResp.ReferenceID)
		fmt.Fprintf(w, "پرداخت شما با موفقیت انجام شد. شماره پیگیری: %s", verifyResp.ReferenceID)

		// تراکنش استفاده شده را از دیتابیس موقت پاک می‌کنیم
		delete(paymentDB, r.FormValue("token"))

	} else {
		// پرداخت ناموفق بوده
		log.Printf("پرداخت ناموفق. وضعیت: %v", verifyResp.Status)
		fmt.Fprintf(w, "پرداخت ناموفق بود. وضعیت: %v", verifyResp.Status)
	}
}
