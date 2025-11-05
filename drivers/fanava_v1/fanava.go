package fanava_v1

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"gopay"
	"io"
	"net/http"
	"strconv"
	"time"
)

// آدرس‌های API بر اساس مستندات
const (
	fanavaGenerateTokenEndpoint = "https://fcp.shaparak.ir/ref-payment/RestServices/mts/generateTokenWithNoSign/"
	fanavaVerifyEndpoint        = "https://fcp.shaparak.ir/ref-payment/RestServices/mts/verifyMerchantTrans/"
	fanavaPaymentEndpoint       = "https://fep.shaparak.ir/ipgw//payment/"
)

// FanavaDriver ساختار اصلی درایور فن‌آوا
type FanavaDriver struct {
	UserID     string
	Password   string
	HttpClient *http.Client
}

// wsContext ساختار مورد نیاز برای احراز هویت در تمام درخواست‌ها
type wsContext struct {
	UserID   string `json:"UserId"`
	Password string `json:"Password"`
}

// --- Purchase (Generate Token) ---

type generateTokenRequest struct {
	WSContext   wsContext `json:"WSContext"`
	TransType   string    `json:"TransType"`  // e.g., "EN_GOODS"
	ReserveNum  string    `json:"ReserveNum"` // شماره فاکتور شما
	Amount      string    `json:"Amount"`     // مبلغ به صورت رشته‌ای
	RedirectURL string    `json:"RedirectUrl"`
}

type generateTokenResponse struct {
	Result         string `json:"Result"` // "erSucceed" (موفق)
	ExpirationDate int64  `json:"ExpirationDate"`
	Token          string `json:"Token"`
	// سایر فیلدها در صورت نیاز
}

// --- Verify (Verify Transaction) ---

type verifyRequest struct {
	WSContext wsContext `json:"WSContext"`
	Token     string    `json:"Token"`
	RefNum    string    `json:"RefNum"`
}

type verifyResponse struct {
	Result string `json:"Result"` // "erSucceed" (موفق)
	Amount int64  `json:"Amount"` // مبلغ
	RefNum string `json:"RefNum"`
}

// NewFanava یک سازنده (InitializerFunc) برای ثبت در کلاینت
func NewFanava(config gopay.DriverConfig) (gopay.Driver, error) {
	uid, ok := config["userID"]
	if !ok {
		return nil, fmt.Errorf("fanava: userID is not set in config")
	}
	pass, ok := config["password"]
	if !ok {
		return nil, fmt.Errorf("fanava: password is not set in config")
	}

	return &FanavaDriver{
		UserID:     uid,
		Password:   pass,
		HttpClient: &http.Client{Timeout: 20 * time.Second},
	}, nil
}

// GetName نام درایور را برمی‌گرداند
func (f *FanavaDriver) GetName() string {
	return "fanava"
}

// Purchase متد پرداخت، توکن را دریافت و کاربر را برای هدایت آماده می‌کند
func (f *FanavaDriver) Purchase(ctx context.Context, req *gopay.TransactionRequest) (*gopay.PaymentResponse, error) {
	// ساخت بدنه درخواست به درگاه
	apiReq := generateTokenRequest{
		WSContext: wsContext{
			UserID:   f.UserID,
			Password: f.Password,
		},
		TransType:   "EN_GOODS",
		ReserveNum:  req.IdempotencyKey, // استفاده از IdempotencyKey به عنوان شماره فاکتور
		Amount:      strconv.FormatInt(req.Amount, 10),
		RedirectURL: req.CallbackURL,
	}

	// ارسال درخواست به سرور فن‌آوا
	respBody, err := f.sendRequest(ctx, fanavaGenerateTokenEndpoint, apiReq)
	if err != nil {
		return nil, err
	}

	// پارس کردن پاسخ
	var respData generateTokenResponse
	if err := json.Unmarshal(respBody, &respData); err != nil {
		return nil, &gopay.GatewayError{
			Code:    -1,
			Message: "Failed to parse Fanava response",
			Err:     err,
		}
	}

	// بررسی خطای دریافتی از API
	if respData.Result != "erSucceed" {
		return nil, &gopay.GatewayError{
			Code:    0, // TODO: Map Fanava errors
			Message: respData.Result,
		}
	}

	if respData.Token == "" {
		return nil, &gopay.GatewayError{Code: -1, Message: "Token was empty"}
	}

	// آماده‌سازی پاسخ برای هدایت کاربر
	// مستندات می‌گوید کاربر باید با متد POST به همراه توکن به درگاه هدایت شود
	return &gopay.PaymentResponse{
		Success:        true,
		Message:        "Token generated successfully",
		Authority:      respData.Token, // توکن را به عنوان شناسه تراکنش (Authority) ذخیره می‌کنیم
		PaymentURL:     fanavaPaymentEndpoint,
		RedirectMethod: "POST",
		RedirectParams: map[string]string{
			"token":    respData.Token,
			"language": "fa",
		},
	}, nil
}

// VerifyAndConfirm تراکنش را پس از بازگشت کاربر از درگاه، تأیید نهایی می‌کند
func (f *FanavaDriver) VerifyAndConfirm(ctx context.Context, r *http.Request, fetcher gopay.TransactionFetcher) (*gopay.VerificationResponse, error) {
	// پارامترهای بازگشتی از درگاه (طبق مستندات)
	// فرض می‌کنیم درگاه پارامترها را با متد POST برمی‌گرداند
	if err := r.ParseForm(); err != nil {
		return nil, &gopay.GatewayError{Code: -1, Message: "Failed to parse callback form", Err: err}
	}

	token := r.FormValue("token")
	refNum := r.FormValue("RefNum")
	state := r.FormValue("State")

	if state != "OK" {
		// تراکنش توسط کاربر لغو شده یا ناموفق بوده
		return &gopay.VerificationResponse{
			Status:       gopay.StatusCancelled, // یا StatusFailed
			ReferenceID:  refNum,
			OriginalData: map[string]interface{}{"callback_form": r.Form},
		}, nil
	}

	if token == "" || refNum == "" {
		return nil, &gopay.GatewayError{Code: -1, Message: "Invalid callback data (token or refNum is missing)"}
	}

	// دریافت اطلاعات تراکنش اصلی از دیتابیس (که در مرحله Purchase ذخیره کردیم)
	originalTx, err := fetcher(ctx, token) // از توکن به عنوان Authority استفاده کردیم
	if err != nil {
		return nil, &gopay.GatewayError{Code: -1, Message: "Transaction fetch failed", Err: err}
	}

	// ساخت درخواست Verify
	apiReq := verifyRequest{
		WSContext: wsContext{
			UserID:   f.UserID,
			Password: f.Password,
		},
		Token:  token,
		RefNum: refNum,
	}

	// ارسال درخواست Verify
	respBody, err := f.sendRequest(ctx, fanavaVerifyEndpoint, apiReq)
	if err != nil {
		return nil, err
	}

	// پارس کردن پاسخ Verify
	var respData verifyResponse
	if err := json.Unmarshal(respBody, &respData); err != nil {
		return nil, &gopay.GatewayError{Code: -1, Message: "Failed to parse Fanava verify response", Err: err}
	}

	// بررسی خطای دریافتی از API
	if respData.Result != "erSucceed" {
		return &gopay.VerificationResponse{
			Status:       gopay.StatusFailed,
			ReferenceID:  refNum,
			OriginalData: map[string]interface{}{"verify_response": respData},
		}, nil
	}

	// بررسی تطابق مبلغ
	if respData.Amount != originalTx.Amount {
		// ! مهم: در این سناریو باید تراکنش را Reverse کرد
		// TODO: Implement Refund (reverseMerchantTrans)
		return &gopay.VerificationResponse{
			Status:       gopay.StatusAmountMismatch,
			ReferenceID:  refNum,
			OriginalData: map[string]interface{}{"verify_response": respData},
		}, nil
	}

	// تراکنش موفق و تایید شده است
	return &gopay.VerificationResponse{
		Status:       gopay.StatusSuccess,
		ReferenceID:  respData.RefNum,
		CardNumber:   "", // فن آوا شماره کارت را در Verify برنمی‌گرداند
		OriginalData: map[string]interface{}{"verify_response": respData},
	}, nil
}

// sendRequest یک متد کمکی برای ارسال درخواست‌های JSON
func (f *FanavaDriver) sendRequest(ctx context.Context, url string, reqBody interface{}) ([]byte, error) {
	body, err := json.Marshal(reqBody)
	if err != nil {
		return nil, &gopay.GatewayError{Code: -1, Message: "Failed to marshal request", Err: err}
	}

	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewBuffer(body))
	if err != nil {
		return nil, &gopay.GatewayError{Code: -1, Message: "Failed to create HTTP request", Err: err}
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")

	resp, err := f.HttpClient.Do(req)
	if err != nil {
		return nil, &gopay.GatewayError{Code: -1, Message: "Failed to send request", Err: err}
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, &gopay.GatewayError{Code: -1, Message: "Failed to read response body", Err: err}
	}

	if resp.StatusCode != http.StatusOK {
		return nil, &gopay.GatewayError{
			Code:    resp.StatusCode,
			Message: fmt.Sprintf("HTTP Error %d: %s", resp.StatusCode, string(respBody)),
		}
	}

	return respBody, nil
}
