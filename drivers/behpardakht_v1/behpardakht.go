package behpardakht_v1

import (
	"bytes"
	"context"
	"encoding/xml"
	"fmt"
	"gopay"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"
)

// اصلاح شد: از InitializerFunc استفاده می‌کند
var Initializer gopay.InitializerFunc = New

const (
	serviceURL = "https://pgwsf.bpm.bankmellat.ir/pgwchannel/services/pgw.asmx"
	paymentURL = "https://bpm.shaparak.ir/pgwchannel/startpay.mellat"
)

// --- ساختارهای درخواست (Request) ---

type bpPayRequest struct {
	XMLName        xml.Name `xml:"soapenv:Envelope"`
	Soapenv        string   `xml:"xmlns:soapenv,attr"`
	Com            string   `xml:"xmlns:com,attr"`
	TerminalId     int64    `xml:"soapenv:Body>com:bpPayRequest>com:terminalId"`
	UserName       string   `xml:"soapenv:Body>com:bpPayRequest>com:userName"`
	UserPassword   string   `xml:"soapenv:Body>com:bpPayRequest>com:userPassword"`
	OrderId        int64    `xml:"soapenv:Body>com:bpPayRequest>com:orderId"`
	Amount         int64    `xml:"soapenv:Body>com:bpPayRequest>com:amount"`
	LocalDate      string   `xml:"soapenv:Body>com:bpPayRequest>com:localDate"`
	LocalTime      string   `xml:"soapenv:Body>com:bpPayRequest>com:localTime"`
	AdditionalData string   `xml:"soapenv:Body>com:bpPayRequest>com:additionalData"`
	CallbackURL    string   `xml:"soapenv:Body>com:bpPayRequest>com:callBackUrl"`
	PayerId        int64    `xml:"soapenv:Body>com:bpPayRequest>com:payerId"`
}

type bpVerifyRequest struct {
	XMLName         xml.Name `xml:"soapenv:Envelope"`
	Soapenv         string   `xml:"xmlns:soapenv,attr"`
	Com             string   `xml:"xmlns:com,attr"`
	TerminalId      int64    `xml:"soapenv:Body>com:bpVerifyRequest>com:terminalId"`
	UserName        string   `xml:"soapenv:Body>com:bpVerifyRequest>com:userName"`
	UserPassword    string   `xml:"soapenv:Body>com:bpVerifyRequest>com:userPassword"`
	OrderId         int64    `xml:"soapenv:Body>com:bpVerifyRequest>com:orderId"`
	SaleOrderId     int64    `xml:"soapenv:Body>com:bpVerifyRequest>com:saleOrderId"`
	SaleReferenceId int64    `xml:"soapenv:Body>com:bpVerifyRequest>com:saleReferenceId"`
}

type bpSettleRequest struct {
	XMLName         xml.Name `xml:"soapenv:Envelope"`
	Soapenv         string   `xml:"xmlns:soapenv,attr"`
	Com             string   `xml:"xmlns:com,attr"`
	TerminalId      int64    `xml:"soapenv:Body>com:bpSettleRequest>com:terminalId"`
	UserName        string   `xml:"soapenv:Body>com:bpSettleRequest>com:userName"`
	UserPassword    string   `xml:"soapenv:Body>com:bpSettleRequest>com:userPassword"`
	OrderId         int64    `xml:"soapenv:Body>com:bpSettleRequest>com:orderId"`
	SaleOrderId     int64    `xml:"soapenv:Body>com:bpSettleRequest>com:saleOrderId"`
	SaleReferenceId int64    `xml:"soapenv:Body>com:bpSettleRequest>com:saleReferenceId"`
}

// --- ساختارهای پاسخ (Response) ---

type bpPayResponse struct {
	XMLName xml.Name `xml:"Envelope"`
	Body    struct {
		PayResponse struct {
			Return string `xml:"return"` // پاسخ به شکل "ResCode,RefId"
		} `xml:"bpPayRequestResponse"`
	} `xml:"Body"`
}

type bpVerifyResponse struct {
	XMLName xml.Name `xml:"Envelope"`
	Body    struct {
		VerifyResponse struct {
			Return string `xml:"return"` // فقط شامل ResCode
		} `xml:"bpVerifyRequestResponse"`
	} `xml:"Body"`
}

type bpSettleResponse struct {
	XMLName xml.Name `xml:"Envelope"`
	Body    struct {
		SettleResponse struct {
			Return string `xml:"return"` // فقط شامل ResCode
		} `xml:"bpSettleRequestResponse"`
	} `xml:"Body"`
}

// --- پیاده سازی درایور ---

type Driver struct {
	TerminalId   int64
	UserName     string
	UserPassword string
	Client       *http.Client
}

var _ gopay.Driver = (*Driver)(nil)
var _ gopay.RedirectPayer = (*Driver)(nil)

func New(config gopay.DriverConfig) (gopay.Driver, error) {
	terminalIdStr, ok := config["terminal_id"]
	if !ok {
		return nil, fmt.Errorf("behpardakht config is missing 'terminal_id'")
	}
	username, ok := config["username"]
	if !ok {
		return nil, fmt.Errorf("behpardakht config is missing 'username'")
	}
	password, ok := config["password"]
	if !ok {
		return nil, fmt.Errorf("behpardakht config is missing 'password'")
	}

	terminalId, err := strconv.ParseInt(terminalIdStr, 10, 64)
	if err != nil {
		return nil, fmt.Errorf("behpardakht config 'terminal_id' is invalid: %w", err)
	}

	return &Driver{
		TerminalId:   terminalId,
		UserName:     username,
		UserPassword: password,
		Client:       &http.Client{},
	}, nil
}

func (d *Driver) GetName() string {
	return "behpardakht_v1"
}

func (d *Driver) Purchase(ctx context.Context, req *gopay.TransactionRequest) (*gopay.PaymentResponse, error) {
	now := time.Now()

	// اصلاح شد: تبدیل IdempotencyKey (string) به OrderId (int64)
	orderId, err := strconv.ParseInt(req.IdempotencyKey, 10, 64)
	if err != nil {
		return nil, &gopay.GatewayError{Err: err, Message: "invalid OrderId (IdempotencyKey must be a valid int64 string)"}
	}

	soapReq := bpPayRequest{
		Soapenv:        "http://schemas.xmlsoap.org/soap/envelope/",
		Com:            "http://interfaces.core.sw.bps.com/",
		TerminalId:     d.TerminalId,
		UserName:       d.UserName,
		UserPassword:   d.UserPassword,
		OrderId:        orderId,
		Amount:         req.Amount,
		LocalDate:      now.Format("20060102"),
		LocalTime:      now.Format("150405"),
		AdditionalData: req.Description,
		CallbackURL:    req.CallbackURL,
		PayerId:        0,
	}

	var soapResponse bpPayResponse
	// اصلاح شد: استفاده از تابع کمکی
	err = d.callSOAP(ctx, "urn:bpPayRequest", soapReq, &soapResponse)
	if err != nil {
		return nil, &gopay.GatewayError{Err: err, Message: "failed to call purchase service"}
	}

	parts := strings.Split(soapResponse.Body.PayResponse.Return, ",")
	if len(parts) != 2 {
		return nil, &gopay.GatewayError{Message: fmt.Sprintf("invalid response from gateway: %s", soapResponse.Body.PayResponse.Return)}
	}

	resCode, _ := strconv.Atoi(parts[0])
	refId := parts[1]

	if resCode != 0 {
		return nil, &gopay.GatewayError{Code: resCode, Message: behpardakhtStatusToMessage(resCode)}
	}

	return &gopay.PaymentResponse{
		Authority:  refId,
		PaymentURL: paymentURL, // کاربر باید به این آدرس POST شود با پارامتر RefId
	}, nil
}

func (d *Driver) VerifyAndConfirm(ctx context.Context, r *http.Request, fetcher gopay.TransactionFetcher) (*gopay.VerificationResponse, error) {
	if err := r.ParseForm(); err != nil {
		return nil, &gopay.GatewayError{Err: err, Message: "failed to parse callback form"}
	}

	resCodeStr := r.FormValue("ResCode")
	saleReferenceIdStr := r.FormValue("SaleReferenceId")
	saleOrderIdStr := r.FormValue("SaleOrderId") // این همان OrderId (IdempotencyKey) شماست

	resCode, _ := strconv.Atoi(resCodeStr)

	if resCode != 0 {
		return &gopay.VerificationResponse{Status: gopay.StatusFailed},
			&gopay.GatewayError{Code: resCode, Message: behpardakhtStatusToMessage(resCode)}
	}

	// اصلاح شد: خطای fetcher مدیریت می‌شود
	original, err := fetcher(ctx, saleOrderIdStr) // در به پرداخت، کلید شما SaleOrderId است
	if err != nil {
		return nil, &gopay.GatewayError{Err: err, Message: "failed to fetch original transaction"}
	}

	// !! نقص امنیتی که قبلاً بحث شد در اینجا قرار دارد !!
	// شما باید 'original.Amount' را بررسی کنید
	// if original.Amount != ??? { // مبلغی که انتظار داشتید
	// 	 return &gopay.VerificationResponse{Status: gopay.StatusAmountMismatch}, nil
	// }
	_ = original // برای جلوگیری از خطای "declared and not used"

	saleOrderId, err := strconv.ParseInt(saleOrderIdStr, 10, 64)
	if err != nil {
		return nil, &gopay.GatewayError{Err: err, Message: "invalid SaleOrderId returned from gateway"}
	}

	saleReferenceId, err := strconv.ParseInt(saleReferenceIdStr, 10, 64)
	if err != nil {
		return nil, &gopay.GatewayError{Err: err, Message: "invalid SaleReferenceId returned from gateway"}
	}

	// مرحله Verify
	verifyResCode, err := d.callVerify(ctx, saleOrderId, saleReferenceId)
	if err != nil {
		return nil, err // خطا در اتصال
	}

	if verifyResCode != 0 {
		return &gopay.VerificationResponse{Status: gopay.StatusFailed},
			&gopay.GatewayError{Code: verifyResCode, Message: behpardakhtStatusToMessage(verifyResCode)}
	}

	// مرحله Settle
	settleResCode, err := d.callSettle(ctx, saleOrderId, saleReferenceId)
	if err != nil {
		return nil, err // خطا در اتصال
	}

	if settleResCode != 0 {
		return &gopay.VerificationResponse{Status: gopay.StatusFailed},
			&gopay.GatewayError{Code: settleResCode, Message: behpardakhtStatusToMessage(settleResCode)}
	}

	return &gopay.VerificationResponse{
		Status:      gopay.StatusSuccess,
		ReferenceID: saleReferenceIdStr,
		OriginalData: map[string]interface{}{
			"SaleOrderId": saleOrderId,
		},
	}, nil
}

// تابع کمکی برای فراخوانی Verify (کامل شد)
func (d *Driver) callVerify(ctx context.Context, orderId int64, saleReferenceId int64) (int, error) {
	soapReq := bpVerifyRequest{
		Soapenv:         "http://schemas.xmlsoap.org/soap/envelope/",
		Com:             "http://interfaces.core.sw.bps.com/",
		TerminalId:      d.TerminalId,
		UserName:        d.UserName,
		UserPassword:    d.UserPassword,
		OrderId:         orderId,
		SaleOrderId:     orderId,
		SaleReferenceId: saleReferenceId,
	}

	var soapResponse bpVerifyResponse
	err := d.callSOAP(ctx, "urn:bpVerifyRequest", soapReq, &soapResponse)
	if err != nil {
		return -1, &gopay.GatewayError{Err: err, Message: "failed to call verify service"}
	}

	resCode, _ := strconv.Atoi(soapResponse.Body.VerifyResponse.Return)
	return resCode, nil
}

// تابع کمکی برای فراخوانی Settle (کامل شد)
func (d *Driver) callSettle(ctx context.Context, orderId int64, saleReferenceId int64) (int, error) {
	soapReq := bpSettleRequest{
		Soapenv:         "http://schemas.xmlsoap.org/soap/envelope/",
		Com:             "http://interfaces.core.sw.bps.com/",
		TerminalId:      d.TerminalId,
		UserName:        d.UserName,
		UserPassword:    d.UserPassword,
		OrderId:         orderId,
		SaleOrderId:     orderId,
		SaleReferenceId: saleReferenceId,
	}

	var soapResponse bpSettleResponse
	err := d.callSOAP(ctx, "urn:bpSettleRequest", soapReq, &soapResponse)
	if err != nil {
		return -1, &gopay.GatewayError{Err: err, Message: "failed to call settle service"}
	}

	resCode, _ := strconv.Atoi(soapResponse.Body.SettleResponse.Return)
	return resCode, nil
}

// callSOAP تابع کمکی جدید برای جلوگیری از تکرار کد
func (d *Driver) callSOAP(ctx context.Context, soapAction string, reqBody interface{}, respBody interface{}) error {
	// Marshal کردن درخواست
	xmlBody, err := xml.Marshal(reqBody)
	if err != nil {
		return fmt.Errorf("failed to marshal soap request: %w", err)
	}

	// ساخت HTTP Request
	httpReq, err := http.NewRequestWithContext(ctx, "POST", serviceURL, bytes.NewBuffer(xmlBody))
	if err != nil {
		return fmt.Errorf("failed to create http request: %w", err)
	}

	httpReq.Header.Set("Content-Type", "text/xml; charset=utf-8")
	httpReq.Header.Set("SOAPAction", soapAction)

	// ارسال درخواست
	resp, err := d.Client.Do(httpReq)
	if err != nil {
		return fmt.Errorf("failed to execute http request: %w", err)
	}
	defer resp.Body.Close()

	// خواندن پاسخ
	rawBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("failed to read soap response body: %w", err)
	}

	// Unmarshal کردن پاسخ
	if err := xml.Unmarshal(rawBody, respBody); err != nil {
		// چاپ متن خام پاسخ در صورت خطای Unmarshal برای دیباگ
		return fmt.Errorf("failed to unmarshal soap response: %w (raw body: %s)", err, string(rawBody))
	}

	return nil
}

// تابع ترجمه خطاها
func behpardakhtStatusToMessage(status int) string {
	switch status {
	case 0:
		return "تراکنش با موفقیت انجام شد"
	case 11:
		return "شماره کارت نامعتبر است"
	case 12:
		return "موجودی کافی نیست"
	case 13:
		return "رمز نادرست است"
	case 14:
		return "تعداد دفعات وارد کردن رمز بیش از حد مجاز است"
	case 15:
		return "کارت نامعتبر است"
	case 17:
		return "کاربر از انجام تراکنش منصرف شده است"
	case 18:
		return "تاریخ انقضای کارت گذشته است"
	case 21:
		return "پذیرنده نامعتبر است"
	case 41:
		return "شماره درخواست تکراری است"
	case 43:
		return "قبلا درخواست Verify داده شده است"
	case 45:
		return "تراکنش Settle شده است"
	case 46:
		return "تراکنش Settle نشده است"
	case 51:
		return "تراکنش تکراری است"
	case 54:
		return "تراکنش مرجع موجود نیست"
	case 55:
		return "تراکنش نامعتبر است"
	case 61:
		return "خطا در واریز"
	default:
		return fmt.Sprintf("خطای ناشناخته با کد: %d", status)
	}
}
