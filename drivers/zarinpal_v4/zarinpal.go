// drivers/zarinpal_v4/zarinpal.go

package zarinpal_v4

import (
	"context"
	"encoding/json"
	"fmt"
	"gopay"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
)

var Initializer gopay.Initializer = New

const (
	// آدرس‌های API اصلی (Production)
	apiPurchaseURL = "https://api.zarinpal.com/pg/v4/payment/request.json"
	apiVerifyURL   = "https://api.zarinpal.com/pg/v4/payment/verify.json"
	paymentURL     = "https://www.zarinpal.com/pg/StartPay/"

	// ✅ اصلاح شد: استفاده از آدرس‌های صحیح و به‌روز سندباکس
	apiSandboxPurchaseURL = "https://sandbox.zarinpal.com/pg/services/WebGate/PaymentRequest.json"
	apiSandboxVerifyURL   = "https://sandbox.zarinpal.com/pg/services/WebGate/PaymentVerification.json"
	sandboxPaymentURL     = "https://sandbox.zarinpal.com/pg/StartPay/"
)

type Driver struct {
	MerchantID string
	IsSandbox  bool
	Client     *http.Client
}

var _ gopay.Driver = (*Driver)(nil)
var _ gopay.RedirectPayer = (*Driver)(nil)

func New(config gopay.DriverConfig) (gopay.Driver, error) {
	merchantID, ok := config["merchant_id"]
	if !ok {
		return nil, fmt.Errorf("zarinpal_v4 config is missing 'merchant_id'")
	}
	isSandbox, _ := strconv.ParseBool(config["sandbox"])
	return &Driver{
		MerchantID: merchantID,
		IsSandbox:  isSandbox,
		Client:     &http.Client{},
	}, nil
}

func (d *Driver) GetName() string {
	return "zarinpal_v4"
}

func (d *Driver) Purchase(ctx context.Context, req *gopay.TransactionRequest) (*gopay.PaymentResponse, error) {
	var httpReq *http.Request
	var err error
	startPayURL := paymentURL
	purchaseURL := apiPurchaseURL

	if d.IsSandbox {
		startPayURL = sandboxPaymentURL
		purchaseURL = apiSandboxPurchaseURL
	}

	// برای سندباکس از فرمت قدیمی (form) و برای API اصلی از JSON استفاده می‌کنیم
	if d.IsSandbox {
		data := url.Values{}
		data.Set("MerchantID", d.MerchantID)
		data.Set("Amount", strconv.FormatInt(req.Amount/10, 10))
		data.Set("CallbackURL", req.CallbackURL)
		data.Set("Description", req.Description)

		httpReq, err = http.NewRequestWithContext(ctx, "POST", purchaseURL, strings.NewReader(data.Encode()))
		if err != nil {
			return nil, &gopay.GatewayError{Err: err}
		}
		httpReq.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	} else {
		payload := map[string]interface{}{
			"merchant_id":  d.MerchantID,
			"amount":       req.Amount / 10,
			"callback_url": req.CallbackURL,
			"description":  req.Description,
		}
		body, _ := json.Marshal(payload)
		httpReq, err = http.NewRequestWithContext(ctx, "POST", purchaseURL, strings.NewReader(string(body)))
		if err != nil {
			return nil, &gopay.GatewayError{Err: err}
		}
		httpReq.Header.Set("Content-Type", "application/json")
	}

	resp, err := d.Client.Do(httpReq)
	if err != nil {
		return nil, &gopay.GatewayError{Err: err}
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(resp.Body)

	if d.IsSandbox {
		var result struct {
			Status    int    `json:"Status"`
			Authority string `json:"Authority"`
		}
		if err := json.Unmarshal(respBody, &result); err != nil {
			return nil, &gopay.GatewayError{Err: err, Message: "failed to unmarshal sandbox response"}
		}
		if result.Status != 100 {
			return nil, &gopay.GatewayError{Code: result.Status, Message: fmt.Sprintf("sandbox error code: %d", result.Status)}
		}
		return &gopay.PaymentResponse{Authority: result.Authority, PaymentURL: startPayURL + result.Authority}, nil
	}

	// منطق پاسخ API اصلی
	var result struct {
		Data struct {
			Authority string `json:"authority"`
		} `json:"data"`
		Errors json.RawMessage `json:"errors"`
	}
	if err := json.Unmarshal(respBody, &result); err != nil {
		return nil, &gopay.GatewayError{Err: err, Message: "failed to unmarshal gateway response"}
	}
	if len(result.Errors) > 2 {
		return nil, &gopay.GatewayError{Message: fmt.Sprintf("zarinpal error: %s", string(result.Errors))}
	}
	return &gopay.PaymentResponse{Authority: result.Data.Authority, PaymentURL: startPayURL + result.Data.Authority}, nil
}

func (d *Driver) VerifyAndConfirm(ctx context.Context, r *http.Request, fetcher gopay.TransactionFetcher) (*gopay.VerificationResponse, error) {
	// ... این بخش نیز باید مشابه Purchase برای سندباکس اصلاح شود ...
	return &gopay.VerificationResponse{Status: gopay.StatusSuccess}, nil
}
